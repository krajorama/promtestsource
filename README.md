# About

This is a very simple prometheus metrics target that can be scraped.

After start it let's the user set a new value for the gauge that it exposes for prometheus.

# Usage

Just use

*go run promtestsource.go*

or

*go run promtestsource.go :5002*

to set a different port and run multiple instances.

# Test

Fetch the current metrics manually with

*curl http://localhost:5001/metrics*

# Why

Simple way to trigger alerts and test them.