
beforeAll(() => {
    // Set the default timeout to 30s to prevent heavy operations
    // to reach the timeout, like creating multiple blocks
    jasmine.DEFAULT_TIMEOUT_INTERVAL = 30 * 1000;
});
