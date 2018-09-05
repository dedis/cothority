package service

/*
A service interacts with the outer world through an API that defines the
methods that are callable from the outside. The service can hold data that
will survive a restart of the cothority and instantiate any number of
protocols before returning a value.

This service defines two methods in the API:
- Clock starts the 'template'-protocol and returns the run-time
- Count returns how many times the 'template'-protocol has been run
*/
