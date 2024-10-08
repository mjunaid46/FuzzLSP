import re

def identify_function_blocks(file_path):
    with open(file_path, 'r') as file:
        lines = file.readlines()

    function_blocks = []
    function_start_pattern = re.compile(r'^[a-zA-Z_][a-zA-Z0-9_]*\s+[a-zA-Z_][a-zA-Z0-9_]*\s*\([^;]*\)\s*{?')
    function_name_pattern = re.compile(r'([a-zA-Z_][a-zA-Z0-9_]*)\s*\(')
    brace_count = 0
    function_start = None
    function_name = None
    inside_function = False
    multiline_comment = False

    for i, line in enumerate(lines):
        stripped_line = line.strip()

        # Handle multiline comments
        if multiline_comment:
            if '*/' in stripped_line:
                multiline_comment = False
            continue
        if '/*' in stripped_line and '*/' not in stripped_line:
            multiline_comment = True
            continue

        # Handle single line comments and preprocessor directives
        if stripped_line.startswith('//') or stripped_line.startswith('#'):
            continue

        if not inside_function:
            # Check for function definition start
            if function_start_pattern.match(stripped_line):
                function_start = i + 1
                function_name_match = function_name_pattern.search(stripped_line)
                if function_name_match:
                    function_name = function_name_match.group(1)
                inside_function = True
                brace_count += stripped_line.count('{') - stripped_line.count('}')
                continue

        if inside_function:
            brace_count += stripped_line.count('{') - stripped_line.count('}')
            if brace_count == 0:
                function_blocks.append((function_name, function_start, i + 1))
                inside_function = False

    return function_blocks

def print_function_blocks(file_path):
    function_blocks = identify_function_blocks(file_path)
    for function_name, start, end in function_blocks:
        print(f"Function '{function_name}' from line {start} to line {end}")

# file_path = r'C:\Users\PMYLS\Desktop\Zortik\Fuzz-LSP\submodules-test\nano\src\color.c'
file_path = r"C:\Users\PMYLS\Desktop\Zortik\Fuzz-LSP\submodules-test\nano\src\utils.c"
# file_path = r"C:\Users\PMYLS\Desktop\Zortik\Fuzz-LSP\submodules-test\nano\src\rcfile.c"

print_function_blocks(file_path)