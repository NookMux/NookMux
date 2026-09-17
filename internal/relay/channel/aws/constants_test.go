package aws

import "testing"

func TestClaudeOpus4ModelMapping(t *testing.T) {
	modelID := getAwsModelID("claude-opus-4-20250514")
	if modelID != "anthropic.claude-opus-4-20250514-v1:0" {
		t.Fatalf("getAwsModelID = %q, want anthropic.claude-opus-4-20250514-v1:0", modelID)
	}

	for _, regionPrefix := range []string{"us"} {
		if !awsModelCanCrossRegion(modelID, regionPrefix) {
			t.Fatalf("awsModelCanCrossRegion(%q, %q) = false, want true", modelID, regionPrefix)
		}
	}
}

func TestClaudeOpus48ModelMappingAndInferenceProfilePrefixing(t *testing.T) {
	t.Parallel()

	if got := getAwsModelID("claude-opus-4-8"); got != "anthropic.claude-opus-4-8" {
		t.Fatalf("getAwsModelID = %q, want anthropic.claude-opus-4-8", got)
	}
	if awsModelCanCrossRegion("us.anthropic.claude-opus-4-8", "us") {
		t.Fatal("native inference profile must not be prefixed again")
	}
	if awsModelCanCrossRegion("arn:aws:bedrock:us-east-1:123:inference-profile/example", "us") {
		t.Fatal("inference profile ARN must not be prefixed")
	}
	if !isAwsInferenceProfileID("us.amazon.nova-pro-v1:0") {
		t.Fatal("native Nova inference profile should be recognized")
	}
}
