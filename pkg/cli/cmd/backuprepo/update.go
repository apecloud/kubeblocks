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
	"reflect"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stoewer/go-strcase"
	"github.com/xeipuuv/gojsonschema"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/cli/util/flags"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

var (
	updateExample = templates.Examples(`
	# Update the credential of a S3-based backuprepo
	kbcli backuprepo update my-backuprepo --access-key-id=<NEW ACCESS KEY> --secret-access-key=<NEW SECRET KEY>

	# Set the backuprepo as default
	kbcli backuprepo update my-backuprepo --default

	# Unset the default backuprepo
	kbcli backuprepo update my-backuprepo --default=false
	`)
)

type updateOptions struct {
	genericiooptions.IOStreams
	dynamic dynamic.Interface
	client  kubernetes.Interface
	factory cmdutil.Factory

	repo            *dpv1alpha1.BackupRepo
	storageProvider string
	providerObject  *storagev1alpha1.StorageProvider
	isDefault       bool
	hasDefaultFlag  bool
	repoName        string
	config          map[string]string
	credential      map[string]string
	allValues       map[string]string
}

func newUpdateCommand(o *updateOptions, f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	if o == nil {
		o = &updateOptions{}
	}
	o.IOStreams = streams

	cmd := &cobra.Command{
		Use:               "update BACKUP_REPO_NAME",
		Short:             "Update a backup repository.",
		Example:           updateExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.BackupRepoGVR()),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.init(f))
			err := o.parseFlags(cmd, args, f)
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
	cmd.Flags().BoolVar(&o.isDefault, "default", false, "Specify whether to set the created backup repo as default")

	return cmd
}

func (o *updateOptions) init(f cmdutil.Factory) error {
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

func (o *updateOptions) parseFlags(cmd *cobra.Command, args []string, f cmdutil.Factory) error {
	// Since we disabled the flag parsing of the cmd, we need to parse it from args
	help := false
	tmpFlags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	tmpFlags.BoolVarP(&help, "help", "h", false, "") // eat --help and -h
	tmpFlags.ParseErrorsWhitelist.UnknownFlags = true
	_ = tmpFlags.Parse(args)
	tmpArgs := tmpFlags.Args()
	if len(tmpArgs) == 0 {
		if help {
			cmd.Long = templates.LongDesc(`
                Note: This help information only shows the common flags for updating a
                backup repository, to show provider-specific flags, please specify
                the name of the backup repository to update. For example:

                    kbcli backuprepo update my-backuprepo --help
            `)
			return pflag.ErrHelp
		}
		return fmt.Errorf("please specify the name of the backup repository to update")
	}

	o.repoName = tmpArgs[0]

	// Get the backup repo from API server
	obj, err := o.dynamic.Resource(types.BackupRepoGVR()).Get(
		context.Background(), o.repoName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("backup repository \"%s\" is not found", o.repoName)
		}
		return err
	}
	repo := &dpv1alpha1.BackupRepo{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, repo)
	if err != nil {
		return err
	}
	o.repo = repo

	// Get provider info from API server
	o.storageProvider = repo.Spec.StorageProviderRef
	obj, err = o.dynamic.Resource(types.StorageProviderGVR()).Get(
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

	// Filter out non-credential fields
	filtered := map[string]apiextensionsv1.JSONSchemaProps{}
	for _, name := range provider.Spec.ParametersSchema.CredentialFields {
		if s, ok := provider.Spec.ParametersSchema.OpenAPIV3Schema.Properties[name]; ok {
			filtered[name] = s
		}
	}
	provider.Spec.ParametersSchema.OpenAPIV3Schema.Properties = filtered
	provider.Spec.ParametersSchema.OpenAPIV3Schema.Required = nil // all fields are optional when doing update

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
	if helpFlag != nil && helpFlag.Value.String() == trueVal {
		return pflag.ErrHelp
	}
	defaultFlag := cmd.Flags().Lookup("default")
	if defaultFlag != nil && defaultFlag.Changed {
		o.hasDefaultFlag = true
	}

	return nil
}

func (o *updateOptions) complete(cmd *cobra.Command) error {
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
		// Collect flags explicitly set by user
		fromFlags := flagsToValues(cmd.LocalNonPersistentFlags(), true)
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
	return nil
}

func (o *updateOptions) validate(cmd *cobra.Command) error {
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

	// Check if there are any default backup repo already exists
	if o.isDefault {
		list, err := o.dynamic.Resource(types.BackupRepoGVR()).List(
			context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, item := range list.Items {
			if item.GetName() == o.repoName {
				continue
			}
			if item.GetAnnotations()[dptypes.DefaultBackupRepoAnnotationKey] == trueVal {
				name := item.GetName()
				return fmt.Errorf("there is already a default backup repository \"%s\","+
					" please set \"%s\" as non-default first",
					name, name)
			}
		}
	}

	return nil
}

func (o *updateOptions) updateCredentialSecret() error {
	if len(o.credential) == 0 {
		// nothing to update
		return nil
	}
	if o.repo.Spec.Credential == nil {
		return nil
	}
	secretObj, err := o.client.CoreV1().Secrets(o.repo.Spec.Credential.Namespace).Get(
		context.Background(), o.repo.Spec.Credential.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	original := secretObj.DeepCopy()
	for k, v := range o.credential {
		secretObj.Data[k] = []byte(v)
	}
	if reflect.DeepEqual(original.Data, secretObj.Data) {
		// nothing to update
		return nil
	}
	patchData, err := createPatchData(original, secretObj)
	if err != nil {
		return err
	}
	_, err = o.client.CoreV1().Secrets(o.repo.Spec.Credential.Namespace).Patch(
		context.Background(), o.repo.Spec.Credential.Name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
	return err
}

func (o *updateOptions) updateDefaultAnnotation() error {
	if !o.hasDefaultFlag {
		// nothing to update
		return nil
	}
	original := o.repo.DeepCopy()
	if o.isDefault {
		if o.repo.Annotations == nil {
			o.repo.Annotations = map[string]string{}
		}
		o.repo.Annotations[dptypes.DefaultBackupRepoAnnotationKey] = trueVal
	} else {
		delete(o.repo.Annotations, dptypes.DefaultBackupRepoAnnotationKey)
	}
	if reflect.DeepEqual(original.ObjectMeta, o.repo.ObjectMeta) {
		// nothing to update
		return nil
	}
	patchData, err := createPatchData(original, o.repo)
	if err != nil {
		return err
	}
	_, err = o.dynamic.Resource(types.BackupRepoGVR()).Patch(
		context.Background(), o.repo.Name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
	return err
}

func (o *updateOptions) run() error {
	if err := o.updateCredentialSecret(); err != nil {
		return err
	}

	if err := o.updateDefaultAnnotation(); err != nil {
		return err
	}

	printer.PrintLine(fmt.Sprintf("Updated."))
	return nil
}
