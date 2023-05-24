//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

#WeaviateEnvs: {

	// Which modules to enable in the setup?
	ENABLE_MODULES?: string

	// The endpoint where to reach the transformers module if enabled
	TRANSFORMERS_INFERENCE_API?: string

	// The endpoint where to reach the clip module if enabled
	CLIP_INFERENCE_API?: string

	QNA_INFERENCE_API?: string

	// The endpoint where to reach the img2vec-neural module if enabled
	IMAGE_INFERENCE_API?: string

	// The id of the AWS access key for the desired account.
	AWS_ACCESS_KEY_ID?: string

	// The secret AWS access key for the desired account.
	AWS_SECRET_ACCESS_KEY?: string

	// The path to the secret GCP service account or workload identity file.
	GOOGLE_APPLICATION_CREDENTIALS?: string

	// The name of your Azure Storage account.
	AZURE_STORAGE_ACCOUNT?: string

	// An access key for your Azure Storage account.
	AZURE_STORAGE_KEY?: string

	// A string that includes the authorization information required.
	AZURE_STORAGE_CONNECTION_STRING?: string

	SPELLCHECK_INFERENCE_API?: string
	NER_INFERENCE_API?:        string
	SUM_INFERENCE_API?:        string
	OPENAI_APIKEY?:            string
	HUGGINGFACE_APIKEY?:       string
	COHERE_APIKEY?:            string
	PALM_APIKEY?:              string
}

// SectionName is section name
[SectionName=_]: #WeaviateEnvs
