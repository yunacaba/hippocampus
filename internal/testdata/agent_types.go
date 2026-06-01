package testdata

import (
	"context"
	"fmt"
	"strings"
)

type SheaficationRequest struct {
	Sheafs []string `json:"sheafs"`
}

// tool:response
type SheaficationResponse struct {
	Sheaf string `json:"sheaf"`
}

var SampleSheaficationResponse = &SheaficationResponse{
	Sheaf: "I am now in a sheaf: one, two, three",
}

func Sheaficate(
	ctx context.Context,
	request *SheaficationRequest,
	invocationId string,
) (*SheaficationResponse, error) {
	return &SheaficationResponse{
		Sheaf: "I am now in a sheaf: " + strings.Join(request.Sheafs, ", "),
	}, nil
}

type BifurcationRequest struct {
	Furcates []string `json:"furcates"`
}

// tool:response
type BifurcationResponse struct {
	FurcateOne    string   `json:"furcate_one"`
	FurcateTwo    string   `json:"furcate_two"`
	ExtraFurcates []string `json:"extra_furcates"`
}

var SampleBifurcationResponse = &BifurcationResponse{
	FurcateOne:    "one",
	FurcateTwo:    "two",
	ExtraFurcates: []string{"five", "six"},
}

func Bifurcate(
	ctx context.Context,
	request *BifurcationRequest,
	invocationId string,
) (*BifurcationResponse, error) {
	if len(request.Furcates) < 2 {
		return nil, fmt.Errorf("not enough furcates")
	}
	return &BifurcationResponse{
		FurcateOne:    request.Furcates[0],
		FurcateTwo:    request.Furcates[1],
		ExtraFurcates: request.Furcates[2:],
	}, nil
}

type TestAgentRequest struct {
	Sheafs   []string `json:"sheafs"`
	Furcates []string `json:"furcates"`
}

// agent:response
type TestAgentResponse struct {
	Sheaf         string   `json:"sheaf"`
	FurcateOne    string   `json:"furcate_one"`
	FurcateTwo    string   `json:"furcate_two"`
	ExtraFurcates []string `json:"extra_furcates"`
}

var SampleTestAgentResponse = &TestAgentResponse{
	Sheaf:         "I am now in a sheaf: one, two, three",
	FurcateOne:    "one",
	FurcateTwo:    "two",
	ExtraFurcates: []string{"five", "six"},
}
