# Copyright 2021 The Kubeflow Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Test Vertex AI Custom Job Remote Runner Client module."""

import json
import unittest
from unittest import mock
from google_cloud_pipeline_components.container.v1.gcp_launcher.utils import json_util


class JsonUtilTests(unittest.TestCase):

    def test_recursive_remove_empty(self):
        payload = '{"display_name": "train_deploy1630230", "description": "", "predict_schemata": {"instance_schema_uri": "", "parameters_schema_uri": "", "prediction_schema_uri": ""}, "container_spec": {"image_uri": "us-docker.pkg.dev/cloud-aiplatform/prediction/tf2-cpu.2-3:latest", "command": "", "args": "", "env": "", "ports": "", "predict_route": "", "health_route": ""}, "artifact_uri": "gs://managed-pipeline-test-bugbash/pipeline_root/yangpa/1630225419", "explanation_spec": {"parameters": {}, "metadata": {}}, "encryption_spec": {"kms_key_name":""}, "default_int": 0}'
        payload_json = json.loads(payload, strict=False)
        payload_json_after = json_util.recursive_remove_empty(payload_json)
        self.assertEqual(
            json.dumps(payload_json_after),
            '{"display_name": "train_deploy1630230", "container_spec": {"image_uri": "us-docker.pkg.dev/cloud-aiplatform/prediction/tf2-cpu.2-3:latest"}, "artifact_uri": "gs://managed-pipeline-test-bugbash/pipeline_root/yangpa/1630225419"}'
        )

        payload = '["abc","def", ""]'
        payload_json = json.loads(payload, strict=False)
        payload_json_after = json_util.recursive_remove_empty(payload_json)
        self.assertEqual(json.dumps(payload_json_after), '["abc", "def", ""]')

        payload = '"abc"'
        payload_json = json.loads(payload, strict=False)
        payload_json_after = json_util.recursive_remove_empty(payload_json)
        self.assertEqual(json.dumps(payload_json_after), '"abc"')

    def test_dont_remove_array_with_zero(self):
        payload = '{"explanation_spec":{"parameters":{"sampled_shapley_attribution":{"path_count":7}},"metadata":{"inputs":{"ps_calc_14":{"input_baselines":[0.0],"input_tensor_name":"","encoding":0,"modality":"","indices_tensor_name":"","dense_shape_tensor_name":"","index_feature_mapping":[],"encoded_tensor_name":"","encoded_baselines":[],"group_name":""}},"outputs":{"scores":{"display_name_mapping_key":"classes","output_tensor_name":""}},"feature_attributions_schema_uri":"gs://sample/gcs/path/feature_attributions.yaml"}}}'
        payload_json = json.loads(payload, strict=False)
        payload_json_after = json_util.recursive_remove_empty(payload_json)
        self.assertEqual(
            json.dumps(payload_json_after),
            '{"explanation_spec": {"parameters": {"sampled_shapley_attribution": {"path_count": 7}}, "metadata": {"inputs": {"ps_calc_14": {"input_baselines": [0.0]}}, "outputs": {"scores": {"display_name_mapping_key": "classes"}}, "feature_attributions_schema_uri": "gs://sample/gcs/path/feature_attributions.yaml"}}}'
        )
