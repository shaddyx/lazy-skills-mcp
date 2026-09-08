---
name: error-handling
description: Idiomatic Go error handling with wrapping and custom types.
tags: [errors, oops, go]
version: 1.0
scripts:
  - name: lint.sh
    description: Runs go vet and staticcheck
---

# Error Handling

Idiomatic error handling in Go: wrap with %w, use errors.Is and errors.As,
and define typed errors for domain rejections.

Use goleak to avoid goroutine leak detection confusion.
