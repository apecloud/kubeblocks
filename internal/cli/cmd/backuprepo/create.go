/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package backuprepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stoewer/go-strcase"
	"github.com/xeipuuv/gojsonschema"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/flags"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
)

const (
	providerFlagName = "provider"
)

var (
	allowedAccessMethods = []string{
		string(dpv1alpha1.AccessMethodMount),
		string(dpv1alpha1.AccessMethodTool),
	}
	allowedPVReclaimPolicies = []string{
		string(corev1.PersistentVolumeReclaimRetain),
		string(corev1.PersistentVolumeReclaimDelete),
	}
)

type createOptions struct {
	genericiooptions.IOStreams
	dynamic dynamic.Interface
	client  kubernetes.Interface
	factory cmdutil.Factory

	accessMethod    string
	storageProvider string
	providerObject  *storagev1alpha1.StorageProvider
	isDefault       bool
	pvReclaimPolicy string
	volumeCapacity  string
	repoName        string
	config          map[string]string
	credential      map[string]string
	allValues       map[string]string
}

var backupRepoCreateExamples = templates.Examples(`
    # Create a default backup repo using S3 as the backend
    kbcli backuprepo create \
      --provider s3 \
      --region us-west-1 \
      --bucket test-kb-backup \
      --access-key-id <ACCESS KEY> \
      --secret-access-key <SECRET KEY> \
      --default

    # Create a non-default backup repo with a specified name
    kbcli backuprepo create my-backup-repo \
      --provider s3 \
      --region us-west-1 \
      --bucket test-kb-backup \
      --access-key-id <ACCESS KEY> \
      --secret-access-key <SECRET KEY>
`)

func newCreateCommand(o *createOptions, f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	if o == nil {
		o = &createOptions{}
	}
	o.IOStreams = streams
	cmd := &cobra.Command{
		Use:     "create [NAME]",
		Short:   "Create a backup repo",
		Example: backupRepoCreateExamples,
		RunE: func(cmd *cobra.Command, args []string) error {
			util.CheckErr(o.init(f))
			err := o.parseProviderFlags(cmd, args, f)
			if errors.Is(err, pflag.ErrHelp) {
				return err
			} else {
				util.CheckErr(err)
			}
			util.CheckErr(o.complete(cmd))
			util.CheckErr(o.validate(cmd))
			util.CheckErr(o.run())
			return nil
		},
		DisableFlagParsing: true,
	}
	cmd.Flags().StringVar(&o.accessMethod, "access-method", "",
		fmt.Sprintf("Specify the access method for the backup repository, \"Tool\" is preferred if not specified. options: %q", allowedAccessMethods))
	cmd.Flags().StringVar(&o.storageProvider, providerFlagName, "", "Specify storage provider")
	util.CheckErr(cmd.MarkFlagRequired(providerFlagName))
	cmd.Flags().BoolVar(&o.isDefault, "default", false, "Specify whether to set the created backup repo as default")
	cmd.Flags().StringVar(&o.pvReclaimPolicy, "pv-reclaim-policy", "Retain",
		`Specify the reclaim policy for PVs created by this backup repo, the value can be "Retain" or "Delete"`)
	cmd.Flags().StringVar(&o.volumeCapacity, "volume-capacity", "100Gi",
		`Specify the capacity of the new created PVC"`)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func (o *createOptions) init(f cmdutil.Factory) error {
	var err error
	if o.dynamic, err = f.DynamicClient(); err != nil {
		return err
	}
	if o.client, err = f.KubernetesClientSet(); err != nil {
		return err
	}
	o.factory = f
	return nil
}

func flagsToValues(fs *pflag.FlagSet) map[string]string {
	values := make(map[string]string)
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		val, _ := fs.GetString(f.Name)
		values[f.Name] = val
	})
	return values
}

func (o *createOptions) parseProviderFlags(cmd *cobra.Command, args []string, f cmdutil.Factory) error {
	// Since we disabled the flag parsing of the cmd, we need to parse it from args
	help := false
	tmpFlags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	tmpFlags.StringVar(&o.storageProvider, providerFlagName, "", "")
	tmpFlags.BoolVarP(&help, "help", "h", false, "") // eat --help and -h
	tmpFlags.ParseErrorsWhitelist.UnknownFlags = true
	_ = tmpFlags.Parse(args)
	if o.storageProvider == "" {
		if help {
			cmd.Long = templates.LongDesc(`
                Note: This help information only shows the common flags for creating a 
                backup repository, to show provider-specific flags, please specify 
                the --provider flag. For example:

                    kbcli backuprepo create --provider s3 --help
            `)
			return pflag.ErrHelp
		}
		return fmt.Errorf("please specify the --%s flag", providerFlagName)
	}

	// Get provider info from API server
	obj, err := o.dynamic.Resource(types.StorageProviderGVR()).Get(
		context.Background(), o.storageProvider, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("storage provider \"%s\" is not found", o.storageProvider)
		}
		return err
	}
	provider := &storagev1alpha1.StorageProvider{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, provider)
	if err != nil {
		return err
	}
	o.providerObject = provider

	// Build flags by schema
	if provider.Spec.ParametersSchema != nil &&
		provider.Spec.ParametersSchema.OpenAPIV3Schema != nil {
		// Convert apiextensionsv1.JSONSchemaProps to spec.Schema
		schemaData, err := json.Marshal(provider.Spec.ParametersSchema.OpenAPIV3Schema)
		if err != nil {
			return err
		}
		schema := &spec.Schema{}
		if err = json.Unmarshal(schemaData, schema); err != nil {
			return err
		}
		if err = flags.BuildFlagsBySchema(cmd, schema); err != nil {
			return err
		}
	}

	// Parse dynamic flags
	cmd.DisableFlagParsing = false
	err = cmd.ParseFlags(args)
	if err != nil {
		return err
	}
	helpFlag := cmd.Flags().Lookup("help")
	if helpFlag != nil && helpFlag.Value.String() == "true" {
		return pflag.ErrHelp
	}
	if err := cmd.ValidateRequiredFlags(); err != nil {
		return err
	}

	return nil
}

func (o *createOptions) complete(cmd *cobra.Command) error {
	o.config = map[string]string{}
	o.credential = map[string]string{}
	o.allValues = map[string]string{}
	schema := o.providerObject.Spec.ParametersSchema
	// Construct config and credential map from flags
	if schema != nil && schema.OpenAPIV3Schema != nil {
		credMap := map[string]bool{}
		for _, x := range schema.CredentialFields {
			credMap[x] = true
		}
		fromFlags := flagsToValues(cmd.LocalNonPersistentFlags())
		for name := range schema.OpenAPIV3Schema.Properties {
			flagName := strcase.KebabCase(name)
			if val, ok := fromFlags[flagName]; ok {
				o.allValues[name] = val
				if credMap[name] {
					o.credential[name] = val
				} else {
					o.config[name] = val
				}
			}
		}
	}
	// Set repo name if specified
	positionArgs := cmd.Flags().Args()
	if len(positionArgs) > 0 {
		o.repoName = positionArgs[0]
	}
	return nil
}

func (o *createOptions) supportedAccessMethods() []string {
	var methods []string
	if o.providerObject.Spec.StorageClassTemplate != "" || o.providerObject.Spec.PersistentVolumeClaimTemplate != "" {
		methods = append(methods, string(dpv1alpha1.AccessMethodMount))
	}
	if o.providerObject.Spec.DatasafedConfigTemplate != "" {
		methods = append(methods, string(dpv1alpha1.AccessMethodTool))
	}
	return methods
}

func (o *createOptions) validate(cmd *cobra.Command) error {
	// Validate values by the json schema
	schema := o.providerObject.Spec.ParametersSchema
	if schema != nil && schema.OpenAPIV3Schema != nil {
		schemaLoader := gojsonschema.NewGoLoader(schema.OpenAPIV3Schema)
		docLoader := gojsonschema.NewGoLoader(o.allValues)
		result, err := gojsonschema.Validate(schemaLoader, docLoader)
		if err != nil {
			return err
		}
		if !result.Valid() {
			for _, err := range result.Errors() {
				flagName := strcase.KebabCase(err.Field())
				cmd.Printf("invalid value \"%v\" for \"--%s\": %s\n",
					err.Value(), flagName, err.Description())
			}
			return fmt.Errorf("invalid flags")
		}
	}

	// Validate access method
	supportedAccessMethods := o.supportedAccessMethods()
	if len(supportedAccessMethods) == 0 {
		return fmt.Errorf("invalid provider \"%s\", it doesn't support any access method", o.storageProvider)
	}
	if o.accessMethod != "" && !slices.Contains(supportedAccessMethods, o.accessMethod) {
		return fmt.Errorf("provider \"%s\" doesn't support \"%s\" access method, supported methods: %q",
			o.storageProvider, o.accessMethod, supportedAccessMethods)
	}
	if o.accessMethod == "" {
		// Prefer using AccessMethodTool if it's supported
		if slices.Contains(supportedAccessMethods, string(dpv1alpha1.AccessMethodTool)) {
			o.accessMethod = string(dpv1alpha1.AccessMethodTool)
		} else {
			o.accessMethod = supportedAccessMethods[0]
		}
	}

	// Validate pv reclaim policy
	if !slices.Contains(allowedPVReclaimPolicies, o.pvReclaimPolicy) {
		return fmt.Errorf("invalid --pv-reclaim-policy \"%s\", the value must be one of %q",
			o.pvReclaimPolicy, allowedPVReclaimPolicies)
	}

	// Validate volume capacity
	if _, err := resource.ParseQuantity(o.volumeCapacity); err != nil {
		return fmt.Errorf("invalid --volume-capacity \"%s\", err: %s", o.volumeCapacity, err)
	}

	// Check if the repo already exists
	if o.repoName != "" {
		_, err := o.dynamic.Resource(types.BackupRepoGVR()).Get(
			context.Background(), o.repoName, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf(`BackupRepo "%s" is already exists`, o.repoName)
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Check if there are any default backup repo already exists
	if o.isDefault {
		list, err := o.dynamic.Resource(types.BackupRepoGVR()).List(
			context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range list.Items {
			if item.GetAnnotations()[dptypes.DefaultBackupRepoAnnotationKey] == "true" {
				name := item.GetName()
				return fmt.Errorf("there is already a default backup repo \"%s\","+
					" please don't specify the --default flag,\n"+
					"\tor set \"%s\" as non-default first",
					name, name)
			}
		}
	}

	return nil
}

func (o *createOptions) createCredentialSecret() (*corev1.Secret, error) {
	// if failed to get the namespace of KubeBlocks,
	// then create the secret in the current namespace
	namespace, err := util.GetKubeBlocksNamespace(o.client)
	if err != nil {
		namespace, _, err = o.factory.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return nil, err
		}
	}
	secretData := map[string][]byte{}
	for k, v := range o.credential {
		secretData[k] = []byte(v)
	}
	secretObj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kb-backuprepo-",
			Namespace:    namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}
	return o.client.CoreV1().Secrets(namespace).Create(
		context.Background(), secretObj, metav1.CreateOptions{})
}

func (o *createOptions) buildBackupRepoObject(secret *corev1.Secret) (*unstructured.Unstructured, error) {
	backupRepo := &dpv1alpha1.BackupRepo{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.DPAPIGroup, types.DPAPIVersion),
			Kind:       "BackupRepo",
		},
		Spec: dpv1alpha1.BackupRepoSpec{
			AccessMethod:       dpv1alpha1.AccessMethod(o.accessMethod),
			StorageProviderRef: o.storageProvider,
			PVReclaimPolicy:    corev1.PersistentVolumeReclaimPolicy(o.pvReclaimPolicy),
			VolumeCapacity:     resource.MustParse(o.volumeCapacity),
			Config:             o.config,
		},
	}
	if o.repoName != "" {
		backupRepo.Name = o.repoName
	} else {
		backupRepo.GenerateName = "backuprepo-"
	}
	if secret != nil {
		backupRepo.Spec.Credential = &corev1.SecretReference{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		}
	}
	if o.isDefault {
		backupRepo.Annotations = map[string]string{
			dptypes.DefaultBackupRepoAnnotationKey: "true",
		}
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(backupRepo)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

func (o *createOptions) setSecretOwnership(secret *corev1.Secret, owner *unstructured.Unstructured) error {
	old := secret.DeepCopyObject()
	refs := secret.GetOwnerReferences()
	refs = append(refs, metav1.OwnerReference{
		APIVersion: owner.GetAPIVersion(),
		Kind:       owner.GetKind(),
		Name:       owner.GetName(),
		UID:        owner.GetUID(),
	})
	secret.SetOwnerReferences(refs)
	oldData, err := json.Marshal(old)
	if err != nil {
		return err
	}
	newData, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	patchData, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return err
	}
	_, err = o.client.CoreV1().Secrets(secret.GetNamespace()).Patch(
		context.Background(), secret.Name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
	return err
}

func (o *createOptions) run() error {
	// create secret
	var createdSecret *corev1.Secret
	if len(o.credential) > 0 {
		var err error
		if createdSecret, err = o.createCredentialSecret(); err != nil {
			return fmt.Errorf("create credential secret failed: %w", err)
		}
	}

	rollbackFn := func() {
		// rollback the created secret if the backup repo creation failed
		if createdSecret != nil {
			_ = o.client.CoreV1().Secrets(createdSecret.Namespace).Delete(
				context.Background(), createdSecret.Name, metav1.DeleteOptions{})
		}
	}

	// create backup repo
	backupRepoObj, err := o.buildBackupRepoObject(createdSecret)
	if err != nil {
		rollbackFn()
		return fmt.Errorf("build BackupRepo object failed: %w", err)
	}
	createdBackupRepo, err := o.dynamic.Resource(types.BackupRepoGVR()).Create(
		context.Background(), backupRepoObj, metav1.CreateOptions{})
	if err != nil {
		rollbackFn()
		return fmt.Errorf("create BackupRepo object failed: %w", err)
	}

	// set ownership of the secret to the repo object
	if createdSecret != nil {
		_ = o.setSecretOwnership(createdSecret, backupRepoObj)
	}

	printer.PrintLine(fmt.Sprintf("Successfully create backup repo \"%s\".", createdBackupRepo.GetName()))
	return nil
}

func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		providerFlagName,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, util.GVRToString(types.StorageProviderGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"access-method",
		cobra.FixedCompletions(allowedAccessMethods, cobra.ShellCompDirectiveNoFileComp)))
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"pv-reclaim-policy",
		cobra.FixedCompletions(allowedPVReclaimPolicies, cobra.ShellCompDirectiveNoFileComp)))

	// TODO: support completion for dynamic flags, if possible
}
