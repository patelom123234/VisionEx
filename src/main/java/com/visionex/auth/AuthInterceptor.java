package com.visionex.auth;

import io.grpc.Context;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;

public class AuthInterceptor implements ServerInterceptor {
    private static final Metadata.Key<String> AUTHORIZATION_KEY =
            Metadata.Key.of("Authorization", Metadata.ASCII_STRING_MARSHALLER);

    private final Auth authClient;

    public AuthInterceptor(Auth authClient) {
        this.authClient = authClient;
    }

    @Override
    public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
            ServerCall<ReqT, RespT> call,
            Metadata headers,
            ServerCallHandler<ReqT, RespT> next) {
        String authorization = headers.get(AUTHORIZATION_KEY);
        if (authorization == null) {
            call.close(Status.UNAUTHENTICATED.withDescription("missing authorization token"), new Metadata());
            return new ServerCall.Listener<>() {};
        }
        try {
            String token = BearerTokenExtractor.extract(authorization);
            authClient.verify(token);
        } catch (Exception ex) {
            call.close(Status.UNAUTHENTICATED.withDescription("invalid token: " + ex.getMessage()), new Metadata());
            return new ServerCall.Listener<>() {};
        }
        return Context.current().withValue(Context.key("authToken"), authorization).run(() -> next.startCall(call, headers));
    }
}

