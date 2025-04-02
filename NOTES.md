## TODO
- write tests for each requirment
  - either unit tests and/or integration tests?
- use proper go package structure?
  - might not be necessary, but keep it in mind

## Identifying Issues
- to start, just read over the requirements, then read over the code to understand how it currently works, and made a note of anything in the code that seemed like it would violate the requirements

- step 2: add tests?

## Thoughts
- do we want to get the cumulative counts for domain stats across all iterations, or reset the domain stats after each iteration?
  - might need to ask for clarification on this one?

- for "production ready", maybe we want to do some light refactoring for the sake of maintainability + extensibility?
  - save this for the very end, though, definitely lower priority just getting the functionality right will keeping the code relatively clean

- will need to make sure requests actually contain everything we expect them to
