# Workshop code review quick check

 - Avoid nested conditions.
 - Delete dead code.
 - Normalise symmetries by sticking to doing identical things identically. 
 ```
 // one way to handle errors
 if err := f(); err != nil {
    ...
 }
 
 // one way to handle multiple returns
 val, err := f()
 if err != nil {
    ...
 }
 ...
 ```
 - The coupled code elements, files, and directories should be adjacent. For example, the test data needs to be stored as close as possible to the test.
 - Move variable declaration and initialisation together.
 - Divide large variable expressions into digestable and self-explanatory ones. Use multiple variables if required.
 - Put a blank line between two logically different chunks of code.
 - Comments explain something not obvious from the code. Delete redundant comments.
