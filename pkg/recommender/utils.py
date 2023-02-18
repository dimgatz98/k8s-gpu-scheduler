#!/usr/bin/python3

import recom_pb2
import pandas as pd
import numpy as np


def parse_protobuf_request(request):
    l = []
    for row in request.data:
        l.append([])
        for col in row.internal_array:
            l[-1].append(col)
    return l


def set_request_data(request, data):
    for ind in data.index:
        row = recom_pb2.InternalArray()
        for col in data.columns:
            if pd.isnull(data.loc[ind, col]):
                row.internal_array.append(-1)
            else:
                row.internal_array.append(float(data.loc[ind, col]))
        request.data.append(row)


def parse_protobuf_reply(reply):
    l = []
    for row in reply.result:
        l.append([])
        for col in row.internal_array:
            l[-1].append(col)
    return l


def set_protobuf_reply(data, columns, reply):
    data = np.array(data)
    for el in data:
        reply.result.append(el)
    for col in columns:
        reply.columns.append(col)


def compute_distance_matrix(mask, target, res):
    return np.abs(target * mask - res * mask)


def print_distance(test_set, target, res):
    mask = np.isnan(np.array(test_set))

    errors = compute_distance_matrix(mask, target, res)
    print("Errors:\n", errors)
    print()
    print()

    masked_errors = np.ma.masked_array(errors, ~np.isnan(np.array(test_set)))
    print("Mean error:", np.round(np.mean(masked_errors), 4))
