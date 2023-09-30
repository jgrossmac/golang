package main

import (
	"kustomize_builder/prompts"
)

func main() {

	prompts.IstioOptions()

	// Ask the user to enter some text
	// fmt.Println("Select an option:")
	// fmt.Println("1. Private")
	// fmt.Println("2. Public")
	// fmt.Println("3. Public and private")

	// fmt.Print("Enter your choice: ")
	// var choice string
	// fmt.Scanln(&choice) // Read user input from the console

	// switch choice {
	// case "1":
	// 	fmt.Println("You selected Option 1.")
	// 	// Perform action for Option 1
	// case "2":
	// 	fmt.Println("You selected Option 2.")
	// 	// Perform action for Option 2
	// case "3":
	// 	fmt.Println("You selected Option 3.")
	// 	// Perform action for Option 3
	// default:
	// 	fmt.Println("Invalid choice. Please select a valid option.")
	// }

	// // Read user input from the console
	// scanner := bufio.NewScanner(os.Stdin)
	// scanner.Scan() // use `Scan` to read the next token from input

	// // Get the user input text
	// userInput1 := scanner.Text()
	// userInput2

	// // Open a file in write-only mode. Create the file if it does not exist.
	// file, err := os.Create("output.txt")
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }
	// defer file.Close() // defer closing the file until the surrounding function returns

	// // Write the user input to the file
	// _, err = file.WriteString(userInput1)
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }

	// fmt.Println("Data has been written to the file 'output.txt'.")

	// /////

	// prompts.MyFunction()

	// // Using the struct from the module
	// instance := prompts.MyStruct{
	// 	Field1: 42,
	// 	Field2: "Hello",
	// }
	// fmt.Println("Struct Field1:", instance.Field1)
	// fmt.Println("Struct Field2:", instance.Field2)

}
