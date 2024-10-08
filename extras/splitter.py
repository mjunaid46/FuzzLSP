import re

def identify_code_blocks(file_path):
    with open(file_path, 'r') as file:
        lines = file.readlines()

    # Regular expressions for identifying blocks
    function_pattern = re.compile(r'\b([a-zA-Z_][a-zA-Z0-9_]*)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^)]*\)\s*{')
    loop_pattern = re.compile(r'\b(for|while|do)\b\s*\(.*\)\s*{')
    conditional_pattern = re.compile(r'\b(if|else\s*if|else|switch)\b\s*\(.*\)\s*{')
    block_start_pattern = re.compile(r'{')
    block_end_pattern = re.compile(r'}')

    code_blocks = []
    stack = []

    # Handle comments and preprocessor directives
    inside_multiline_comment = False

    for i, line in enumerate(lines):
        stripped_line = line.strip()

        # Handle multiline comments
        if inside_multiline_comment:
            if '*/' in stripped_line:
                inside_multiline_comment = False
            continue
        if '/*' in stripped_line:
            inside_multiline_comment = True
            continue

        # Handle single line comments and preprocessor directives
        if stripped_line.startswith('//') or stripped_line.startswith('#'):
            continue

        # Check for function definitions
        function_match = function_pattern.search(stripped_line)
        if function_match:
            function_name = function_match.group(2)
            stack.append((i + 1, function_name))
            continue

        # Check for loops
        loop_match = loop_pattern.search(stripped_line)
        if loop_match:
            loop_type = loop_match.group(1)
            stack.append((i + 1, f'{loop_type} loop'))
            continue

        # Check for conditionals
        conditional_match = conditional_pattern.search(stripped_line)
        if conditional_match:
            conditional_type = conditional_match.group(1)
            stack.append((i + 1, f'{conditional_type} statement'))
            continue

        # Check for other block starts
        if block_start_pattern.search(stripped_line) and not function_match and not loop_match and not conditional_match:
            stack.append((i + 1, 'block'))

        # Check for block ends
        if block_end_pattern.search(stripped_line):
            if stack:
                start_line, block_name = stack.pop()
                code_blocks.append((block_name, start_line, i + 1))

    return code_blocks

def print_code_blocks(file_path):
    code_blocks = identify_code_blocks(file_path)
    for block_name, start, end in code_blocks:
        print(f"{block_name} from line {start} to line {end}")

file_path = r'C:\Users\PMYLS\Desktop\Zortik\Fuzz-LSP\submodules-test\nano\src\color.c'
print_code_blocks(file_path)
