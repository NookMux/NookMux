package aws

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	bedrockTypes "github.com/aws/aws-sdk-go-v2/service/bedrock/types"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type awsRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn awsRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestParseAwsCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rawKey     string
		keyType    shared.AwsKeyType
		wantMode   awsCredentialMode
		wantAPIKey string
		wantAccess string
		wantSecret string
		wantRegion string
		wantErr    string
	}{
		{name: "api key", rawKey: " bearer-token | us-east-1 ", keyType: shared.AwsKeyTypeApiKey, wantMode: awsCredentialModeAPIKey, wantAPIKey: "bearer-token", wantRegion: "us-east-1"},
		{name: "access key", rawKey: "access|secret|us-west-2", keyType: shared.AwsKeyTypeAKSK, wantMode: awsCredentialModeAKSK, wantAccess: "access", wantSecret: "secret", wantRegion: "us-west-2"},
		{name: "legacy api key", rawKey: "token|eu-west-1", wantMode: awsCredentialModeAPIKey, wantAPIKey: "token", wantRegion: "eu-west-1"},
		{name: "legacy access key", rawKey: "access|secret|ap-southeast-1", wantMode: awsCredentialModeAKSK, wantAccess: "access", wantSecret: "secret", wantRegion: "ap-southeast-1"},
		{name: "invalid region", rawKey: "token|not-a-region", keyType: shared.AwsKeyTypeApiKey, wantErr: "invalid AWS region"},
		{name: "wrong api key shape", rawKey: "access|secret|us-east-1", keyType: shared.AwsKeyTypeApiKey, wantErr: "invalid AWS API key, expected APIKey|Region"},
		{name: "wrong access key shape", rawKey: "token|us-east-1", keyType: shared.AwsKeyTypeAKSK, wantErr: "invalid AWS access key, expected AccessKey|SecretAccessKey|Region"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credential, err := parseAwsCredential(tt.rawKey, tt.keyType)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("parseAwsCredential error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAwsCredential: %v", err)
			}
			if credential.mode != tt.wantMode || credential.apiKey != tt.wantAPIKey || credential.accessKeyID != tt.wantAccess || credential.secretAccessKey != tt.wantSecret || credential.region != tt.wantRegion {
				t.Fatalf("credential = %#v, want mode=%v api=%q access=%q secret=%q region=%q", credential, tt.wantMode, tt.wantAPIKey, tt.wantAccess, tt.wantSecret, tt.wantRegion)
			}
		})
	}
}

func TestNewAwsClientsUseSelectedAuthentication(t *testing.T) {
	t.Parallel()

	apiCredential, err := parseAwsCredential("only-the-token|us-east-1", shared.AwsKeyTypeApiKey)
	if err != nil {
		t.Fatal(err)
	}
	apiRuntimeClient := newAwsRuntimeClient(apiCredential, http.DefaultClient)
	token, err := apiRuntimeClient.Options().BearerAuthTokenProvider.RetrieveBearerToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token.Value != "only-the-token" || apiRuntimeClient.Options().Credentials != nil {
		t.Fatalf("API key client authentication is not configured correctly")
	}

	accessCredential, err := parseAwsCredential("access|secret|us-east-1", shared.AwsKeyTypeAKSK)
	if err != nil {
		t.Fatal(err)
	}
	accessClient := newAwsRuntimeClient(accessCredential, http.DefaultClient)
	credentials, err := accessClient.Options().Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if credentials.AccessKeyID != "access" || credentials.SecretAccessKey != "secret" || accessClient.Options().BearerAuthTokenProvider != nil {
		t.Fatalf("access key client authentication is not configured correctly")
	}
}

func TestAwsSDKClientsSendExpectedAuthorization(t *testing.T) {
	t.Parallel()

	captureClient := func(authorization *string, responseBody string) *http.Client {
		return &http.Client{Transport: awsRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			*authorization = request.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(responseBody)),
				Request:    request,
			}, nil
		})}
	}

	apiCredential, err := parseAwsCredential("only-the-token|us-east-1", shared.AwsKeyTypeApiKey)
	if err != nil {
		t.Fatal(err)
	}
	controlAuthorization := ""
	controlClient := newAwsBedrockClient(apiCredential, captureClient(&controlAuthorization, `{"modelSummaries":[]}`), func(options *bedrock.Options) {
		options.BaseEndpoint = aws.String("https://bedrock.test")
	})
	_, err = controlClient.ListFoundationModels(context.Background(), &bedrock.ListFoundationModelsInput{
		ByProvider:       aws.String("Anthropic"),
		ByOutputModality: bedrockTypes.ModelModalityText,
		ByInferenceType:  bedrockTypes.InferenceTypeOnDemand,
	})
	if err != nil {
		t.Fatal(err)
	}
	if controlAuthorization != "Bearer only-the-token" {
		t.Fatalf("control plane authorization = %q", controlAuthorization)
	}

	runtimeAuthorization := ""
	runtimeClient := newAwsRuntimeClient(apiCredential, captureClient(&runtimeAuthorization, `{}`), func(options *bedrockruntime.Options) {
		options.BaseEndpoint = aws.String("https://bedrock-runtime.test")
	})
	_, err = runtimeClient.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("us.anthropic.claude-sonnet-4-6"),
		ContentType: aws.String("application/json"),
		Body:        []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if runtimeAuthorization != "Bearer only-the-token" {
		t.Fatalf("runtime authorization = %q", runtimeAuthorization)
	}

	accessCredential, err := parseAwsCredential("access|secret|us-east-1", shared.AwsKeyTypeAKSK)
	if err != nil {
		t.Fatal(err)
	}
	signedAuthorization := ""
	signedClient := newAwsBedrockClient(accessCredential, captureClient(&signedAuthorization, `{"modelSummaries":[]}`), func(options *bedrock.Options) {
		options.BaseEndpoint = aws.String("https://bedrock.test")
	})
	if _, err = signedClient.ListFoundationModels(context.Background(), &bedrock.ListFoundationModelsInput{}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(signedAuthorization, "AWS4-HMAC-SHA256 ") || !strings.Contains(signedAuthorization, "Credential=access/") {
		t.Fatalf("signature authorization = %q", signedAuthorization)
	}
}
