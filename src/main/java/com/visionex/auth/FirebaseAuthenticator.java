package com.visionex.auth;

import com.google.firebase.auth.FirebaseAuth;
import com.google.firebase.auth.FirebaseToken;
import java.util.List;
import java.util.Locale;

public class FirebaseAuthenticator implements Auth {
    private static final List<String> VALID_EMAIL_DOMAINS = List.of(
            "yanolja.com",
            "ezeetechnosys.com",
            "yanoljacloudsolution.com",
            "interparktriple.com",
            "nol-universe.com",
            "goglobal.travel",
            "group.yanolja.com"
    );

    private final FirebaseAuth firebaseAuth;

    public FirebaseAuthenticator(FirebaseAuth firebaseAuth) {
        this.firebaseAuth = firebaseAuth;
    }

    @Override
    public String verify(String token) throws Exception {
        FirebaseToken decoded = firebaseAuth.verifyIdToken(token);
        Object emailClaim = decoded.getClaims().get("email");
        if (!(emailClaim instanceof String)) {
            throw new Exception("failed to verify the token: invalid email in claim");
        }
        String email = (String) emailClaim;
        if (!email.contains("@")) {
            throw new Exception("failed to verify the token: invalid email format");
        }
        String[] parts = email.split("@");
        if (parts.length != 2) {
            throw new Exception("failed to verify the token: malformed email structure (expected single '@')");
        }
        String domain = parts[1].toLowerCase(Locale.ROOT);
        if (!VALID_EMAIL_DOMAINS.contains(domain)) {
            throw new Exception("failed to verify the token: invalid email domain");
        }
        return token;
    }
}

