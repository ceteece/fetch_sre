## TODO
- write tests for each requirment
  - either unit tests and/or integration tests?
- use proper go package structure?
  - might not be necessary, but keep it in mind

## Identifying Issues
- to start, just read over the requirements, then read over the code to understand how it currently works, and made a note of anything in the code that seemed like it would violate the requirements

- step 2: add tests?

- issues:
  - `extractDomain` makes mistakes in some cases:
    - identified issues by reading the code
    - confirmed by writing test cases to cover suspected failure cases (and other basic test cases to prevent regression)
    - issues with original code:
      - included port numbers in domain
      - if path contained `//`, would not parse the domain correctly (it would use the part of the path after the last `//` as the domain)
    - updated code to ensure all test cases passed

  - `checkHealth` would not report a failure if request took more than 500 ms:
    - identified issue by reading the code
    - confirmed issue by adding test cases for `checkHealth`, one of which included the server taking more than 500 ms to respond
    - added `Timeout` value of 500 ms to `http.Client` used by `checkHealth`, now `checkHealth` cancels the request after 500 ms and reports that the request failed

  - when checking all endpoints the `checkHealth` calls are serialized, which could easily take more than 15 seconds if the total latency of all of the endpoints is high enough
    - moved logic for checking all endpoints to `checkEndpoints` function, which checks health of each endpoint exactly once
    - added `TestCheckEndpoints` test to confirm that, with starter code, time to check all endpoints exceeds 15s with 100 endpoints with 250ms latency each
    - updated `checkEndpoints` function to check all endpoints in parallel, with each `checkHealth` call in a separate goroutine, ensuring we can check a large number of endpoints while staying well with the 15s interval
      - also updated `DomainStats` to use atomic integers to prevent race conditions when multiple `checkHealth` calls are run in parallel for endpoints belonging to the same domain

  - `checkHealth` was sending entire `Endpoint` struct as request body, should only be sending actual body
    - discovered while writing "TestPostOK", when adding check to confirm that body matches what is provided in the YAML file
    - fixed by updating `checkHealth` to only use the actual body as the request body

  - we sleep for 15 seconds after all calls to `checkHealth`, which means our actual health check period will exceed 15s
    - identified by reading the code, confirmed by adding test case that fails when checking health of all endpoints takes significant amount of time (`TestSlow`)
    - updated code to wait to log stats until 15 seconds have passed since the stats were last logged, rather than 15 seconds since the stats were last collected


## Thoughts
- do we want to get the cumulative counts for domain stats across all iterations, or reset the domain stats after each iteration?
  - might need to ask for clarification on this one?

- for "production ready", maybe we want to do some light refactoring for the sake of maintainability + extensibility?
  - save this for the very end, though, definitely lower priority just getting the functionality right will keeping the code relatively clean

- will need to make sure requests actually contain everything we expect them to

- will need to serialize updates to counters for domain stats

- will probably want some end-to-end integration tests

- figuring out how to test some of this stuff might be tricky

- will need to test that YAML file gets parsed properly at some point

- might want to update `checkHealth` to return `healthResults` with two boolean fields `Attempted` and `Succeeded`, or something like that, rather than relying on updating a global variable

- also, might want to not have a global variable and refactor things into one or more structs or something
  - again, though, let's save that for the end, after things are mostly working
    - and even then, we probably should keep changes relatively minimal

- add tests to confirm that even if we have a large number of endpoints (with delays that would take well over 15 seconds if the requests were all serialized), we can hit all of them in under 15 seconds

- don't forget to run `go fmt` to format everything before submitting code
