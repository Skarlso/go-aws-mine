package commands

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"testing"

	"github.com/Yitsushi/go-commander"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/cloudformationiface"
	"github.com/go-furnace/go-furnace/config"
	awsconfig "github.com/go-furnace/go-furnace/furnace-aws/config"
	"github.com/go-furnace/go-furnace/handle"
)

type fakeCreateCFClient struct {
	cloudformationiface.ClientAPI
	stackname string
	err       error
}

func init() {
	handle.LogFatalf = log.Fatalf
}

func (fc *fakeCreateCFClient) ValidateTemplateRequest(input *cloudformation.ValidateTemplateInput) cloudformation.ValidateTemplateRequest {
	return cloudformation.ValidateTemplateRequest{
		Request: &aws.Request{
			Data:        &cloudformation.ValidateTemplateOutput{},
			Error:       fc.err,
			HTTPRequest: new(http.Request),
		},
		Input: input,
	}
}

func (fc *fakeCreateCFClient) CreateStackRequest(input *cloudformation.CreateStackInput) cloudformation.CreateStackRequest {
	return cloudformation.CreateStackRequest{
		Request: &aws.Request{
			Data: &cloudformation.CreateStackOutput{
				StackId: aws.String("DummyID"),
			},
			Error:       fc.err,
			HTTPRequest: new(http.Request),
		},
		Input: input,
	}

}

func (fc *fakeCreateCFClient) WaitUntilStackCreateComplete(ctx context.Context, input *cloudformation.DescribeStacksInput, opts ...aws.WaiterOption) error {
	return nil
}

func (fc *fakeCreateCFClient) DescribeStacksRequest(input *cloudformation.DescribeStacksInput) cloudformation.DescribeStacksRequest {
	if fc.stackname == "NotEmptyStack" || fc.stackname == "DescribeStackFailed" {
		return cloudformation.DescribeStacksRequest{
			Request: &aws.Request{
				Data:        &NotEmptyStack,
				Error:       fc.err,
				HTTPRequest: new(http.Request),
			},
		}
	}
	return cloudformation.DescribeStacksRequest{
		Request: &aws.Request{
			Data:        &cloudformation.DescribeStacksOutput{},
			HTTPRequest: new(http.Request),
		},
	}
}

func TestCreateExecute(t *testing.T) {
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "NotEmptyStack"
	client.Client = &fakeCreateCFClient{err: nil, stackname: stackname}
	opts := &commander.CommandHelper{}
	opts.Args = make([]string, 0)
	opts.Args = append(opts.Args, "teststack")
	c := Create{
		client: client,
	}
	c.Execute(opts)
}

func TestCreateExecuteWithStackFile(t *testing.T) {
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "NotEmptyStack"
	client.Client = &fakeCreateCFClient{err: nil, stackname: stackname}
	opts := &commander.CommandHelper{}
	opts.Args = append(opts.Args, "teststack")
	c := Create{
		client: client,
	}
	c.Execute(opts)
	if awsconfig.Config.Main.Stackname != "MyStack" {
		t.Fatal("test did not load the file requested.")
	}
}

func TestCreateExecuteWithStackFileNotFound(t *testing.T) {
	failed := false
	handle.LogFatalf = func(s string, a ...interface{}) {
		failed = true
	}
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "NotEmptyStack"
	client.Client = &fakeCreateCFClient{err: nil, stackname: stackname}
	opts := &commander.CommandHelper{}
	opts.Args = append(opts.Args, "notpresent")
	c := Create{
		client: client,
	}
	c.Execute(opts)
	if !failed {
		t.Error("Expected outcome to fail. Did not fail.")
	}
}

func TestCreateExecuteEmptyStack(t *testing.T) {
	failed := false
	handle.LogFatalf = func(s string, a ...interface{}) {
		failed = true
	}
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "EmptyStack"
	client.Client = &fakeCreateCFClient{err: nil, stackname: stackname}
	opts := &commander.CommandHelper{}
	c := Create{
		client: client,
	}
	c.Execute(opts)
	if !failed {
		t.Error("Expected outcome to fail. Did not fail.")
	}
}

func TestCreateProcedure(t *testing.T) {
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "NotEmptyStack"
	client.Client = &fakeCreateCFClient{err: nil, stackname: stackname}
	template := []byte("{}")
	stacks := create(stackname, template, client)
	if len(stacks) == 0 {
		t.Fatal("Stack was not returned by create.")
	}
	if *stacks[0].StackName != "TestStack" {
		t.Fatal("Not the correct stack returned. Returned was:", stacks)
	}
}

func TestCreateStackReturnsWithError(t *testing.T) {
	failed := false
	expectedMessage := "the response was nil"
	var message string
	handle.LogFatalf = func(s string, a ...interface{}) {
		failed = true
		message = a[0].(error).Error()
	}
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "NotEmptyStack"
	client.Client = &fakeCreateCFClient{err: errors.New(expectedMessage), stackname: stackname}
	template := []byte("{}")
	create(stackname, template, client)
	if !failed {
		t.Error("Expected outcome to fail. Did not fail.")
	}
	if message != expectedMessage {
		t.Errorf("message did not equal expected message of '%s', was:%s", expectedMessage, message)
	}
}

func TestDescribeStackReturnsWithError(t *testing.T) {
	failed := false
	expected := "the response was nil"
	var message string
	handle.LogFatalf = func(s string, a ...interface{}) {
		failed = true
		if err, ok := a[0].(error); ok {
			message = err.Error()
		}
	}
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "DescribeStackFailed"
	client.Client = &fakeCreateCFClient{err: errors.New(expected), stackname: stackname}
	template := []byte("{}")
	create(stackname, template, client)
	if !failed {
		t.Error("Expected outcome to fail. Did not fail.")
	}
	if message != expected {
		t.Error("message did not equal expected message of 'the response was nil', was:", message)
	}
}

func TestValidateReturnsWithError(t *testing.T) {
	failed := false
	expectedMessage := "the response was nil"
	var message string
	handle.LogFatalf = func(s string, a ...interface{}) {
		failed = true
		if err, ok := a[0].(error); ok {
			message = err.Error()
		}
	}
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "ValidationError"
	client.Client = &fakeCreateCFClient{err: errors.New(expectedMessage), stackname: stackname}
	template := []byte("{}")
	create(stackname, template, client)
	if !failed {
		t.Error("Expected outcome to fail. Did not fail.")
	}
	if message != expectedMessage {
		t.Errorf("message did not equal expected message of '%s', was:%s", expectedMessage, message)
	}
}

func TestCreateReturnsEmptyStack(t *testing.T) {
	config.WAITFREQUENCY = 0
	client := new(CFClient)
	stackname := "EmptyStack"
	client.Client = &fakeCreateCFClient{err: nil, stackname: stackname}
	template := []byte("{}")
	stacks := create(stackname, template, client)
	if len(stacks) != 0 {
		t.Fatal("Stack was not empty: ", stacks)
	}
}

func TestGatheringParametersWithoutSpecifyingUserInputShouldUseDefaultValue(t *testing.T) {
	in, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	validOutput := &cloudformation.ValidateTemplateOutput{
		Parameters: []cloudformation.TemplateParameter{
			{
				DefaultValue: aws.String("DefaultValue"),
				Description:  aws.String("Description"),
				NoEcho:       aws.Bool(false),
				ParameterKey: aws.String("Key"),
			},
		},
	}
	params := gatherParameters(in, validOutput)
	if *params[0].ParameterKey != "Key" {
		t.Fatal("Key did not equal expected key. Was:", *params[0].ParameterKey)
	}
	if *params[0].ParameterValue != "DefaultValue" {
		t.Fatal("Value did not equal expected value. Was:", *params[0].ParameterValue)
	}
}

func TestGatheringParametersWithUserInputShouldUseInput(t *testing.T) {
	// Create a temp file
	in, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	// Write the new value in that file
	_, err = io.WriteString(in, "NewValue\n")
	if err != nil {
		t.Fatal(err)
	}
	// Set the starting point for the next read to be the beginning of the file
	_, err = in.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	// Setup the input
	validOutput := &cloudformation.ValidateTemplateOutput{
		Parameters: []cloudformation.TemplateParameter{
			{
				DefaultValue: aws.String("DefaultValue"),
				Description:  aws.String("Description"),
				NoEcho:       aws.Bool(false),
				ParameterKey: aws.String("Key"),
			},
		},
	}
	params := gatherParameters(in, validOutput)
	if *params[0].ParameterKey != "Key" {
		t.Fatal("Key did not equal expected key. Was:", *params[0].ParameterKey)
	}
	if *params[0].ParameterValue != "NewValue" {
		t.Fatal("Value did not equal expected value. Was:", *params[0].ParameterValue)
	}
}

func TestNewCreate(t *testing.T) {
	wrapper := NewCreate("furnace")
	if wrapper.Help.Arguments != "custom-config" ||
		!reflect.DeepEqual(wrapper.Help.Examples, []string{"", "custom-config"}) ||
		wrapper.Help.LongDescription != `Create a stack on which to deploy code later on. By default FurnaceStack is used as name.` ||
		wrapper.Help.ShortDescription != "Create a stack" {
		t.Log(wrapper.Help.LongDescription)
		t.Log(wrapper.Help.ShortDescription)
		t.Log(wrapper.Help.Examples)
		t.Fatal("wrapper did not match with given params")
	}
}
