---
name: testing
description: Production-ready Go tests with parallel subtests and fuzzing.
tags: [test, table-driven]
scripts:
  - name: setup.sh
    description: Initializes the dev environment
  - name: run.sh
    description: Runs the test suite
---

# Testing

Use table-driven tests with named subtests. Run in parallel with t.Parallel().

Detect goroutine leaks after each test with goleak.
