
beforeAll(() => {
    // Set the default timeout to 60s to prevent heavy operations
    // to reach the timeout, like creating multiple blocks
    jasmine.DEFAULT_TIMEOUT_INTERVAL = 60 * 1000;
});
