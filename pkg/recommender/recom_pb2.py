# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: recom.proto
"""Generated protocol buffer code."""
from google.protobuf.internal import builder as _builder
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x0brecom.proto\x12\x0brecommender\"\x18\n\x07Request\x12\r\n\x05index\x18\x01 \x01(\t\"(\n\x05Reply\x12\x0e\n\x06result\x18\x01 \x03(\x02\x12\x0f\n\x07\x63olumns\x18\x02 \x03(\t2\x8f\x01\n\x0brecommender\x12@\n\x14ImputeConfigurations\x12\x14.recommender.Request\x1a\x12.recommender.Reply\x12>\n\x12ImputeInterference\x12\x14.recommender.Request\x1a\x12.recommender.Replyb\x06proto3')

_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, globals())
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'recom_pb2', globals())
if _descriptor._USE_C_DESCRIPTORS == False:

  DESCRIPTOR._options = None
  _REQUEST._serialized_start=28
  _REQUEST._serialized_end=52
  _REPLY._serialized_start=54
  _REPLY._serialized_end=94
  _RECOMMENDER._serialized_start=97
  _RECOMMENDER._serialized_end=240
# @@protoc_insertion_point(module_scope)