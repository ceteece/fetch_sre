## Installation
In the top-level project directory, run `go mod tidy`

## Usage
To start monitoring endpoints, run:
```
go run main.go <config_file>
```

where `<config_file>` is the path to your YAML configuration file.

## Identifying Issues
### General Approach
To identify issues, I started by reading over the requirements and then reading over the code to understand how it currently works, making note of anything that seemed like it would violate the requirements.

Then, I wrote a combination of automated unit and integrations tests, trying to make sure the test suite covered all of the project requirements. Using these tests I was able to confirm that the issues I identified by reading the code were actually issues, and was also able to discover some issues I hadn't initially seen. I would then update the code as necessary to resolve the issues, and then confirm that the associated test cases passed after the fixes were applied.

### Discovered Issues and Solutions:
#### `extractDomain`
- `extractDomain` makes mistakes in some cases
- identified issues by reading the code
- confirmed by writing test cases (`TestExtractDomain`) to cover suspected failure cases (and other basic test cases to prevent regression)
- issues with original code:
  - included port numbers in domain
  - if path contained `//`, would not parse the domain correctly (it would use the part of the path after the last `//` as the domain)
- updated `extractDomain` to specifically look for and remove `http://` or `https://` prefix, instead of splitting on `//`
- updated `extractDomain` to remove port numbers if present, by splitting the authority section of the URL on `:` and only keeping the string before the first `:`

#### `checkHealth`
- `checkHealth` would not report a failure if request took more than 500 ms:
  - identified issue by reading the code
  - confirmed issue by adding test cases for `checkHealth` (`TestCheckHealth`), one of which included the server taking more than 500 ms to respond
  - added `Timeout` value of 500 ms to `http.Client` used by `checkHealth`, now `checkHealth` cancels the request after 500 ms and reports that the request failed

- `checkHealth` was sending entire `Endpoint` struct as request body, should only be sending actual body
  - discovered while writing `TestPostOK` test, when adding check to confirm that body matches what is provided in the YAML file
  - fixed by updating `checkHealth` to only use the actual body as the request body

#### `monitorEndpoints`
- when checking all endpoints the `checkHealth` calls are serialized, which could easily take more than 15 seconds if the total latency of all of the endpoints is high enough
  - moved logic for checking all endpoints to `checkEndpoints` function, which checks health of each endpoint exactly once
  - added `TestCheckEndpoints` test to confirm that, with starter code, time to check all endpoints exceeds 15s with 100 endpoints with 250ms latency each
  - updated `checkEndpoints` function to check all endpoints in parallel, with each `checkHealth` call in a separate goroutine, ensuring we can check a large number of endpoints while staying well with the 15s interval
    - also updated `DomainStats` to use atomic integers to prevent race conditions when multiple `checkHealth` calls are run in parallel for endpoints belonging to the same domain

- we sleep for 15 seconds after all calls to `checkHealth`, which means our actual health check period will exceed 15s (i.e. the actual time period will be `time_to_check_all_endpoints + 15s`)
  - identified by reading the code, confirmed by adding test case that has slow-to-respond server and fails when time interval between consecutive iterations is not between 14600ms and 15400ms (`TestSlow`)
  - updated code to wait to log stats until 15 seconds have passed since the stats were last logged, rather than 15 seconds since the stats were last collected
