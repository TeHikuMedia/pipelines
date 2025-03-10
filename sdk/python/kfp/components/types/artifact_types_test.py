# Copyright 2021 The Kubeflow Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Tests for kfp.components.types.artifact_types."""

import json
import os
import unittest

from absl.testing import parameterized
from kfp.components.types import artifact_types


class ArtifactsTest(parameterized.TestCase):

    def test_complex_metrics(self):
        metrics = artifact_types.ClassificationMetrics()
        metrics.log_roc_data_point(threshold=0.1, tpr=98.2, fpr=96.2)
        metrics.log_roc_data_point(threshold=24.3, tpr=24.5, fpr=98.4)
        metrics.set_confusion_matrix_categories(['dog', 'cat', 'horses'])
        metrics.log_confusion_matrix_row('dog', [2, 6, 0])
        metrics.log_confusion_matrix_cell('cat', 'dog', 3)
        metrics.log_confusion_matrix_cell('horses', 'horses', 3)
        metrics.metadata['test'] = 1.0
        with open(
                os.path.join(
                    os.path.dirname(__file__), 'test_data',
                    'expected_io_types_classification_metrics.json')
        ) as json_file:
            expected_json = json.load(json_file)
            self.assertEqual(expected_json, metrics.metadata)

    def test_complex_metrics_bulk_loading(self):
        metrics = artifact_types.ClassificationMetrics()
        metrics.log_roc_curve(
            fpr=[85.1, 85.1, 85.1],
            tpr=[52.6, 52.6, 52.6],
            threshold=[53.6, 53.6, 53.6])
        metrics.log_confusion_matrix(['dog', 'cat', 'horses'],
                                     [[2, 6, 0], [3, 5, 6], [5, 7, 8]])
        with open(
                os.path.join(
                    os.path.dirname(__file__), 'test_data',
                    'expected_io_types_bulk_load_classification_metrics.json')
        ) as json_file:
            expected_json = json.load(json_file)
            self.assertEqual(expected_json, metrics.metadata)

    @parameterized.parameters(
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.Artifact'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.Artifact,
        },
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.Model'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.Model,
        },
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.Dataset'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.Dataset,
        },
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.Metrics'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.Metrics,
        },
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.ClassificationMetrics'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.ClassificationMetrics,
        },
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.SlicedClassificationMetrics'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.SlicedClassificationMetrics,
        },
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.HTML'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.HTML,
        },
        {
            'runtime_artifact': {
                'metadata': {},
                'name': 'input_artifact_one',
                'type': {
                    'schemaTitle': 'system.Markdown'
                },
                'uri': 'gs://some-bucket/input_artifact_one'
            },
            'expected_type': artifact_types.Markdown,
        },
    )
    def test_create_runtime_artifact(
        self,
        runtime_artifact,
        expected_type,
    ):
        self.assertIsInstance(
            artifact_types.create_runtime_artifact(runtime_artifact),
            expected_type)


if __name__ == '__main__':
    unittest.main()
