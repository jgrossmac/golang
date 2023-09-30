// module.go

package prompts

import "fmt"

type Options struct {
	Selected []string
}

func IstioOptions() {

	options := []string{"Public", "Private"}

	var selectedOptions []string
	prompt := &survey.MultiSelect{
		Message: "Select options:",
		Options: options,
	}
	err := survey.AskOne(prompt, &selectedOptions)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Ingress options:")
	for _, option := range selectedOptions {
		fmt.Println("- " + option)
	}
}
