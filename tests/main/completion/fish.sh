set -ex

# fish completion functions use commands which only work in interactive mode.
# We simulate an interactive session and check the right command was executed.
# This is enough to test that the completion script was installed successfully.
# More complex scenarios can be tested using unit tests or bash.
workshop connections ws-comp | grep -Eq '^mount[[:space:]]+ws-comp/test-sdk-mount:two[[:space:]]+ws-comp/system:mount[[:space:]]+-$'
printf '%s\t\nexit\n' 'workshop disconnect ws-comp/test-sdk-mount:t' | script -qc fish /dev/null > /dev/null
workshop connections ws-comp | grep -Eq '^mount[[:space:]]+ws-comp/test-sdk-mount:two[[:space:]]+-[[:space:]]+-$'
