package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/Yitsushi/go-commander"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/fatih/color"
	"github.com/go-furnace/go-furnace/config"
	awsconfig "github.com/go-furnace/go-furnace/furnace-aws/config"
	"github.com/go-furnace/go-furnace/handle"
)

// Update command.
type Update struct {
}

// Execute defines what this command does.
func (c *Update) Execute(opts *commander.CommandHelper) {
	log.Println("Creating cloud formation session.")
	cfg, err := external.LoadDefaultAWSConfig()
	handle.Error(err)
	cfClient := cloudformation.New(cfg)
	client := CFClient{cfClient}
	updateExecute(opts, &client)
}

func updateExecute(opts *commander.CommandHelper, client *CFClient) {
	configName := opts.Arg(0)
	if len(configName) > 0 {
		dir, _ := os.Getwd()
		if err := awsconfig.LoadConfigFileIfExists(dir, configName); err != nil {
			handle.Fatal(configName, err)
		}
	}
	stackname := awsconfig.Config.Main.Stackname
	template := awsconfig.LoadCFStackConfig()
	stacks := update(stackname, template, client)
	var red = color.New(color.FgRed).SprintFunc()
	if stacks != nil {
		log.Println("Stack state is: ", red(stacks[0].StackStatus))
	} else {
		handle.Fatal(fmt.Sprintf("No stacks found with name: %s", keyName(stackname)), nil)
	}
}

func update(stackname string, template []byte, cfClient *CFClient) []cloudformation.Stack {
	validResp := cfClient.validateTemplate(template)
	stackParameters := gatherParameters(os.Stdin, validResp)
	stackInputParams := &cloudformation.UpdateStackInput{
		StackName: aws.String(stackname),
		Capabilities: []cloudformation.Capability{
			cloudformation.CapabilityCapabilityIam,
		},
		Parameters:   stackParameters,
		TemplateBody: aws.String(string(template)),
	}
	resp := cfClient.updateStack(stackInputParams)
	log.Println("Update stack response: ", resp)
	cfClient.waitForStackUpdateComplete(stackname)
	descResp := cfClient.describeStacks(&cloudformation.DescribeStacksInput{StackName: aws.String(stackname)})
	fmt.Println()
	if descResp == nil {
		return nil
	}
	return descResp.Stacks
}

func (cf *CFClient) waitForStackUpdateComplete(stackname string) {
	describeStackInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackname),
	}
	waitForFunctionWithStatusOutput("UPDATE_COMPLETE", config.WAITFREQUENCY, func() {
		cf.Client.WaitUntilStackUpdateComplete(describeStackInput)
	})
}

func (cf *CFClient) updateStack(stackInputParams *cloudformation.UpdateStackInput) *cloudformation.UpdateStackOutput {
	log.Println("Updating Stack with name: ", keyName(*stackInputParams.StackName))
	req := cf.Client.UpdateStackRequest(stackInputParams)
	resp, err := req.Send()
	handle.Error(err)
	return resp
}

// NewUpdate Updates a new Update command.
func NewUpdate(appName string) *commander.CommandWrapper {
	return &commander.CommandWrapper{
		Handler: &Update{},
		Help: &commander.CommandDescriptor{
			Name:             "update",
			ShortDescription: "Update a stack",
			LongDescription:  `Update a stack with new parameters.`,
			Arguments:        "custom-config",
			Examples:         []string{"", "custom-config"},
		},
	}
}
