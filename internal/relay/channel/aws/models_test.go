package aws

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	bedrockTypes "github.com/aws/aws-sdk-go-v2/service/bedrock/types"
)

type mockBedrockModelLister struct {
	profilePages      []*bedrock.ListInferenceProfilesOutput
	profileErr        error
	foundationOutput  *bedrock.ListFoundationModelsOutput
	foundationErr     error
	profileCalls      int
	foundationRequest *bedrock.ListFoundationModelsInput
}

func (m *mockBedrockModelLister) ListInferenceProfiles(_ context.Context, input *bedrock.ListInferenceProfilesInput, _ ...func(*bedrock.Options)) (*bedrock.ListInferenceProfilesOutput, error) {
	if m.profileErr != nil {
		return nil, m.profileErr
	}
	if input.TypeEquals != bedrockTypes.InferenceProfileTypeSystemDefined {
		return nil, errors.New("unexpected profile type")
	}
	if m.profileCalls >= len(m.profilePages) {
		return &bedrock.ListInferenceProfilesOutput{}, nil
	}
	page := m.profilePages[m.profileCalls]
	m.profileCalls++
	return page, nil
}

func (m *mockBedrockModelLister) ListFoundationModels(_ context.Context, input *bedrock.ListFoundationModelsInput, _ ...func(*bedrock.Options)) (*bedrock.ListFoundationModelsOutput, error) {
	m.foundationRequest = input
	if m.foundationErr != nil {
		return nil, m.foundationErr
	}
	return m.foundationOutput, nil
}

func TestFetchClaudeModelsPrefersActiveInferenceProfiles(t *testing.T) {
	t.Parallel()

	client := &mockBedrockModelLister{
		profilePages: []*bedrock.ListInferenceProfilesOutput{
			{
				InferenceProfileSummaries: []bedrockTypes.InferenceProfileSummary{
					{
						InferenceProfileId: aws.String("us.anthropic.claude-sonnet-4-6"),
						Status:             bedrockTypes.InferenceProfileStatusActive,
						Models: []bedrockTypes.InferenceProfileModel{{
							ModelArn: aws.String("arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-sonnet-4-6"),
						}},
					},
					{
						InferenceProfileId: aws.String("us.amazon.nova-pro-v1:0"),
						Status:             bedrockTypes.InferenceProfileStatusActive,
					},
				},
				NextToken: aws.String("page-2"),
			},
			{
				InferenceProfileSummaries: []bedrockTypes.InferenceProfileSummary{
					{
						InferenceProfileId: aws.String("global.anthropic.claude-haiku-4-5-20251001-v1:0"),
						Status:             bedrockTypes.InferenceProfileStatusActive,
						Models: []bedrockTypes.InferenceProfileModel{{
							ModelArn: aws.String("arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-haiku-4-5-20251001-v1:0"),
						}},
					},
				},
			},
		},
		foundationOutput: &bedrock.ListFoundationModelsOutput{
			ModelSummaries: []bedrockTypes.FoundationModelSummary{
				activeClaudeFoundationModel("anthropic.claude-sonnet-4-6"),
				activeClaudeFoundationModel("anthropic.claude-haiku-4-5-20251001-v1:0"),
				activeClaudeFoundationModel("anthropic.claude-3-haiku-20240307-v1:0"),
				legacyClaudeFoundationModel("anthropic.claude-2-v1:0"),
			},
		},
	}

	models, err := fetchClaudeModels(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"anthropic.claude-3-haiku-20240307-v1:0",
		"global.anthropic.claude-haiku-4-5-20251001-v1:0",
		"us.anthropic.claude-sonnet-4-6",
	}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	if client.profileCalls != 2 {
		t.Fatalf("profile calls = %d, want 2", client.profileCalls)
	}
	if got := aws.ToString(client.foundationRequest.ByProvider); got != "Anthropic" {
		t.Fatalf("provider filter = %q", got)
	}
	if client.foundationRequest.ByOutputModality != bedrockTypes.ModelModalityText || client.foundationRequest.ByInferenceType != bedrockTypes.InferenceTypeOnDemand {
		t.Fatalf("foundation filters = %#v", client.foundationRequest)
	}
}

func TestFetchClaudeModelsReturnsControlPlaneErrors(t *testing.T) {
	t.Parallel()

	if _, err := fetchClaudeModels(context.Background(), &mockBedrockModelLister{profileErr: errors.New("access denied")}); err == nil || err.Error() != "fetch AWS Claude inference profiles failed: access denied" {
		t.Fatalf("profile error = %v", err)
	}
	if _, err := fetchClaudeModels(context.Background(), &mockBedrockModelLister{
		profilePages:  []*bedrock.ListInferenceProfilesOutput{{}},
		foundationErr: errors.New("throttled"),
	}); err == nil || err.Error() != "fetch AWS Claude foundation models failed: throttled" {
		t.Fatalf("foundation error = %v", err)
	}
}

func TestFoundationModelIDFromARN(t *testing.T) {
	t.Parallel()
	if got := foundationModelIDFromARN("arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-sonnet-4-6"); got != "anthropic.claude-sonnet-4-6" {
		t.Fatalf("foundation model id = %q", got)
	}
	if got := foundationModelIDFromARN("arn:aws:bedrock:us-east-1:123:inference-profile/example"); got != "" {
		t.Fatalf("inference profile ARN parsed as %q", got)
	}
}

func activeClaudeFoundationModel(modelID string) bedrockTypes.FoundationModelSummary {
	return bedrockTypes.FoundationModelSummary{
		ModelId:                 aws.String(modelID),
		ProviderName:            aws.String("Anthropic"),
		ModelLifecycle:          &bedrockTypes.FoundationModelLifecycle{Status: bedrockTypes.FoundationModelLifecycleStatusActive},
		OutputModalities:        []bedrockTypes.ModelModality{bedrockTypes.ModelModalityText},
		InferenceTypesSupported: []bedrockTypes.InferenceType{bedrockTypes.InferenceTypeOnDemand},
	}
}

func legacyClaudeFoundationModel(modelID string) bedrockTypes.FoundationModelSummary {
	model := activeClaudeFoundationModel(modelID)
	model.ModelLifecycle.Status = bedrockTypes.FoundationModelLifecycleStatusLegacy
	return model
}
