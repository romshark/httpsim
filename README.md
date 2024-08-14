<a href="https://pkg.go.dev/github.com/romshark/httpsim">
    <img src="https://godoc.org/github.com/romshark/httpsim?status.svg" alt="GoDoc">
</a>
<a href="https://goreportcard.com/report/github.com/romshark/httpsim">
    <img src="https://goreportcard.com/badge/github.com/romshark/httpsim" alt="GoReportCard">
</a>
<a href='https://coveralls.io/github/romshark/httpsim?branch=main'>
    <img src='https://coveralls.io/repos/github/romshark/httpsim/badge.svg?branch=main' alt='Coverage Status' />
</a>

# httpsim

Package httpsim provides an HTTP latency and response simulator middleware
that simplifies conditionally adding artificial delays and overwriting responses
matching requests by path, headers and query parameters using glob expressions.

```yaml
resources:
  # Make DELETE requests at path "/specific" return 404 responses (overwrite).
  - path: /specific
    methods: [DELETE] # DELETE requests only
    effect:
      replace:
        status-code: 404
        body: "Specific resource not found"
        headers:
          Content-Type: text/plain
          X-Custom: custom
  - path: /* # This is a glob expression for anything behind the root "/".
    # Any HTTP method
    headers:
      # Match requests only when header
      # "Content-Type" exactly equals "application/javascript".
      - name: "Content-Type"
        values: ["application/javascript"] # This is a glob expression.
    query:
      # Match requests only when query parameter
      # "param" starts with "щы".
      # example: "/?param=щы" is matched.
      # example: "/foo/bar?sid=123&param=щы456" is matched.
      # example: "/foo/bar?sid=123&param=щ" is not matched.
      - parameter: "param"
        values: ["щы*"] # This is a glob expression.
    effect:
      # Simulate latency for matched requests by
      # applying a 200-1000 millisecond delay.
      delay:
        min: 200ms
        max: 1s
```

See [github.com/gobwas/glob](https://github.com/gobwas/glob) for how to use globs.
