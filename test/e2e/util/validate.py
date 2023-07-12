#!/usr/bin/env python3
# -*- coding:utf-8 -*-

import glob
import os
import yaml
import jsonschema


def get_current_path():
    return os.getcwd()


def get_files(directory, extension):
    files = glob.glob(os.path.join(directory, f'*{extension}'))
    return [os.path.realpath(file) for file in files]


def load_yamls():
    current_path = get_current_path()
    directory = current_path + "/../../../test/e2e/testdata/smoketest"
    dict_yamls = {}
    for root, dirs, files in os.walk(directory):
        for file in files:
            if file.endswith('.yaml'):
                with open(os.path.join(root, file)) as f:
                    for data in yaml.safe_load_all(f):
                        kind = data["kind"]
                        dict_yamls[kind] = data
    return dict_yamls


def load_crd_schema(filename):
    with open(filename) as f:
        crd = yaml.safe_load(f)
        kind = crd["spec"]["names"]["kind"]
        schema = crd["spec"]["versions"][0]["schema"]["openAPIV3Schema"]
    return kind, schema


def get_schemas():
    schemas = {}
    current_path = get_current_path()
    files = get_files(current_path + "/../../../config/crd/bases", ".yaml")
    for file in files:
        kind, schema = load_crd_schema(file)
        schemas[kind] = schema
    return schemas


if __name__ == "__main__":
    schemas = get_schemas()
    yamls = load_yamls()
    for schema_key in schemas.keys():
        for yaml_key in yamls.keys():
            if schema_key == yaml_key:
                for yaml in yamls.values():
                    jsonschema.validate(schemas.values(), yaml)
    print("test cases validate pass")


