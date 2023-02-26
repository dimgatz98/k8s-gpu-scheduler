# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
"""Client and server classes corresponding to protobuf-defined services."""
import grpc

import recom_pb2 as recom__pb2


class recommenderStub(object):
    """Missing associated documentation comment in .proto file."""

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.ImputeConfigurations = channel.unary_unary(
            "/recommender.recommender/ImputeConfigurations",
            request_serializer=recom__pb2.Request.SerializeToString,
            response_deserializer=recom__pb2.Reply.FromString,
        )
        self.ImputeInterference = channel.unary_unary(
            "/recommender.recommender/ImputeInterference",
            request_serializer=recom__pb2.Request.SerializeToString,
            response_deserializer=recom__pb2.Reply.FromString,
        )


class recommenderServicer(object):
    """Missing associated documentation comment in .proto file."""

    def ImputeConfigurations(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")

    def ImputeInterference(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Method not implemented!")
        raise NotImplementedError("Method not implemented!")


def add_recommenderServicer_to_server(servicer, server):
    rpc_method_handlers = {
        "ImputeConfigurations": grpc.unary_unary_rpc_method_handler(
            servicer.ImputeConfigurations,
            request_deserializer=recom__pb2.Request.FromString,
            response_serializer=recom__pb2.Reply.SerializeToString,
        ),
        "ImputeInterference": grpc.unary_unary_rpc_method_handler(
            servicer.ImputeInterference,
            request_deserializer=recom__pb2.Request.FromString,
            response_serializer=recom__pb2.Reply.SerializeToString,
        ),
    }
    generic_handler = grpc.method_handlers_generic_handler(
        "recommender.recommender", rpc_method_handlers
    )
    server.add_generic_rpc_handlers((generic_handler,))


# This class is part of an EXPERIMENTAL API.
class recommender(object):
    """Missing associated documentation comment in .proto file."""

    @staticmethod
    def ImputeConfigurations(
        request,
        target,
        options=(),
        channel_credentials=None,
        call_credentials=None,
        insecure=False,
        compression=None,
        wait_for_ready=None,
        timeout=None,
        metadata=None,
    ):
        return grpc.experimental.unary_unary(
            request,
            target,
            "/recommender.recommender/ImputeConfigurations",
            recom__pb2.Request.SerializeToString,
            recom__pb2.Reply.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
        )

    @staticmethod
    def ImputeInterference(
        request,
        target,
        options=(),
        channel_credentials=None,
        call_credentials=None,
        insecure=False,
        compression=None,
        wait_for_ready=None,
        timeout=None,
        metadata=None,
    ):
        return grpc.experimental.unary_unary(
            request,
            target,
            "/recommender.recommender/ImputeInterference",
            recom__pb2.Request.SerializeToString,
            recom__pb2.Reply.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
        )
