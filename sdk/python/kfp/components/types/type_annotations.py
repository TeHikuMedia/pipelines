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
"""Classes for input/output type annotations in KFP SDK.

These are only compatible with v2 Pipelines.
"""

import re
from typing import TypeVar, Union

try:
    from typing import Annotated
except ImportError:
    from typing_extensions import Annotated

T = TypeVar('T')


class OutputPath:
    """Type annotation used in component definitions for indicating a parameter
    is a path to an output. The path parameter typed with this annotation can
    be treated as a locally accessible filepath within the component body.

    The argument typed with this annotation is provided at runtime by the executing backend and does not need to be passed as an input by the pipeline author (see example).


    Args:
        type: The type of the value written to the output path.

    Example:
      ::

        @dsl.component
        def create_parameter(
                message: str,
                output_parameter_path: OutputPath(str),
        ):
            with open(output_parameter_path, 'w') as f:
                f.write(message)


        @dsl.component
        def consume_parameter(message: str):
            print(message)


        @dsl.pipeline(name='my-pipeline', pipeline_root='gs://my-bucket')
        def my_pipeline(message: str = 'default message'):
            create_param_op = create_parameter(message=message)
            consume_parameter(message=create_param_op.outputs['output_parameter_path'])
    """

    def __init__(self, type=None):
        self.type = type

    def __eq__(self, other):
        if isinstance(other, OutputPath):
            return self.type == other.type
        return False


class InputPath:
    """Type annotation used in component definitions for indicating a parameter
    is a path to an input.

    Example:
      ::

        @dsl.component
        def create_dataset(dataset_path: OutputPath('Dataset'),):
            import json
            dataset = {'my_dataset': [[1, 2, 3], [4, 5, 6]]}
            with open(dataset_path, 'w') as f:
                json.dump(dataset, f)


        @dsl.component
        def consume_dataset(dataset: InputPath('Dataset')):
            print(dataset)


        @dsl.pipeline(name='my-pipeline', pipeline_root='gs://my-bucket')
        def my_pipeline():
            create_dataset_op = create_dataset()
            consume_dataset(dataset=create_dataset_op.outputs['dataset_path'])
    """

    def __init__(self, type=None):
        self.type = type

    def __eq__(self, other):
        if isinstance(other, InputPath):
            return self.type == other.type
        return False


class InputAnnotation():
    """Marker type for input artifacts."""


class OutputAnnotation():
    """Marker type for output artifacts."""


Input = Annotated[T, InputAnnotation]
Input.__doc__ = """Type generic used to represent an input artifact of type ``T``, where ``T`` is an artifact class.

Use ``Input[Artifact]`` or ``Output[Artifact]`` to indicate whether the enclosed artifact is a component input or output.

Args:
    T: The type of the input artifact.

Example:
  ::

    @dsl.component
    def artifact_producer(model: Output[Artifact]):
        with open(model.path, 'w') as f:
            f.write('my model')

    @dsl.component
    def artifact_consumer(model: Input[Artifact]):
        print(model)

    @dsl.pipeline(name='my-pipeline')
    def my_pipeline():
        producer_task = artifact_producer()
        artifact_consumer(model=producer_task.output)
"""

Output = Annotated[T, OutputAnnotation]
Output.__doc__ = """A type generic used to represent an output artifact of type ``T``, where ``T`` is an artifact class. The argument typed with this annotation is provided at runtime by the executing backend and does not need to be passed as an input by the pipeline author (see example).

Use ``Input[Artifact]`` or ``Output[Artifact]`` to indicate whether the enclosed artifact is a component input or output.

Args:
    T: The type of the output artifact.

Example:
  ::

    @dsl.component
    def artifact_producer(model: Output[Artifact]):
        with open(model.path, 'w') as f:
            f.write('my model')

    @dsl.component
    def artifact_consumer(model: Input[Artifact]):
        print(model)

    @dsl.pipeline(name='my-pipeline')
    def my_pipeline():
        producer_task = artifact_producer()
        artifact_consumer(model=producer_task.output)
"""


def is_artifact_annotation(typ) -> bool:
    if not hasattr(typ, '__metadata__'):
        return False

    if typ.__metadata__[0] not in [InputAnnotation, OutputAnnotation]:
        return False

    return True


def is_input_artifact(typ) -> bool:
    """Returns True if typ is of type Input[T]."""
    if not is_artifact_annotation(typ):
        return False

    return typ.__metadata__[0] == InputAnnotation


def is_output_artifact(typ) -> bool:
    """Returns True if typ is of type Output[T]."""
    if not is_artifact_annotation(typ):
        return False

    return typ.__metadata__[0] == OutputAnnotation


def get_io_artifact_class(typ):
    if not is_artifact_annotation(typ):
        return None
    if typ == Input or typ == Output:
        return None

    return typ.__args__[0]


def get_io_artifact_annotation(typ):
    if not is_artifact_annotation(typ):
        return None

    return typ.__metadata__[0]


def maybe_strip_optional_from_annotation(annotation: T) -> T:
    """Strips 'Optional' from 'Optional[<type>]' if applicable.

    For example::
      Optional[str] -> str
      str -> str
      List[int] -> List[int]

    Args:
      annotation: The original type annotation which may or may not has
        `Optional`.

    Returns:
      The type inside Optional[] if Optional exists, otherwise the original type.
    """
    if getattr(annotation, '__origin__',
               None) is Union and annotation.__args__[1] is type(None):
        return annotation.__args__[0]
    return annotation


def get_short_type_name(type_name: str) -> str:
    """Extracts the short form type name.

    This method is used for looking up serializer for a given type.

    For example::
      typing.List -> List
      typing.List[int] -> List
      typing.Dict[str, str] -> Dict
      List -> List
      str -> str

    Args:
      type_name: The original type name.

    Returns:
      The short form type name or the original name if pattern doesn't match.
    """
    match = re.match('(typing\.)?(?P<type>\w+)(?:\[.+\])?', type_name)
    if match:
        return match.group('type')
    else:
        return type_name
