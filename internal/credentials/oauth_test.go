package credentials

import (
	"strings"
	"testing"
	"time"
)

func TestOAuthTokenRoundTrip(t *testing.T) {
	want := OAuthToken{AccessToken: "access-secret", RefreshToken: "refresh-secret", TokenType: "Bearer", ExpiresAt: time.Unix(1234, 0).UTC()}
	encoded, err := EncodeOAuthToken(want)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "dhocs.v1.") {
		t.Fatal("unexpected client secret in credential")
	}
	got, err := DecodeOAuthToken(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestDecodeOAuthTokenRejectsUnversionedAccessToken(t *testing.T) {
	secret := "unversioned-access-secret"
	_, err := DecodeOAuthToken(secret)
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("credential leaked in error")
	}
}

func TestDecodeOAuthTokenRejectsMalformedBundleWithoutLeakingIt(t *testing.T) {
	secret := oauthCredentialPrefix + `{not-json "access-secret"}`
	_, err := DecodeOAuthToken(secret)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "access-secret") {
		t.Fatal("credential leaked in error")
	}
}

func TestOAuthTokenNeedsRefresh(t *testing.T) {
	now := time.Unix(1000, 0)
	if (OAuthToken{ExpiresAt: now.Add(2 * time.Minute)}).NeedsRefresh(now, time.Minute) {
		t.Fatal("fresh token requires refresh")
	}
	if !(OAuthToken{ExpiresAt: now.Add(time.Minute)}).NeedsRefresh(now, time.Minute) {
		t.Fatal("near-expiry token did not require refresh")
	}
	if (OAuthToken{}).NeedsRefresh(now, time.Minute) {
		t.Fatal("credential without expiry requires refresh")
	}
}
