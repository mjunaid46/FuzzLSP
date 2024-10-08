package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type FunctionBlock struct {
	name  string
	start int
	end   int
}

func identifyFunctionBlocks(filePath string) ([]FunctionBlock, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	functionBlocks := []FunctionBlock{}
	functionStartPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*\s+[a-zA-Z_][a-zA-Z0-9_]*\s*\([^;]*\)\s*{?`)
	functionNamePattern := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	braceCount := 0
	functionStart := 0
	functionName := ""
	insideFunction := false
	multilineComment := false

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		strippedLine := strings.TrimSpace(line)

		// Handle multiline comments
		if multilineComment {
			if strings.Contains(strippedLine, "*/") {
				multilineComment = false
			}
			continue
		}
		if strings.Contains(strippedLine, "/*") && !strings.Contains(strippedLine, "*/") {
			multilineComment = true
			continue
		}

		// Handle single line comments and preprocessor directives
		if strings.HasPrefix(strippedLine, "//") || strings.HasPrefix(strippedLine, "#") {
			continue
		}

		if !insideFunction {
			// Check for function definition start
			if functionStartPattern.MatchString(strippedLine) {
				functionStart = lineNumber
				functionNameMatch := functionNamePattern.FindStringSubmatch(strippedLine)
				if len(functionNameMatch) > 1 {
					functionName = functionNameMatch[1]
				}
				insideFunction = true
				braceCount += strings.Count(strippedLine, "{") - strings.Count(strippedLine, "}")
				continue
			}
		}

		if insideFunction {
			braceCount += strings.Count(strippedLine, "{") - strings.Count(strippedLine, "}")
			if braceCount == 0 {
				functionBlocks = append(functionBlocks, FunctionBlock{name: functionName, start: functionStart, end: lineNumber})
				insideFunction = false
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return functionBlocks, nil
}

func printFunctionBlocks(filePath string) {
	functionBlocks, err := identifyFunctionBlocks(filePath)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	for _, block := range functionBlocks {
		fmt.Printf("Function '%s' from line %d to line %d\n", block.name, block.start, block.end)
	}
}

func main() {
	filePath := "C:\\Users\\PMYLS\\Desktop\\Zortik\\Fuzz-LSP\\submodules-test\\nano\\src\\color.c"
	printFunctionBlocks(filePath)
}
