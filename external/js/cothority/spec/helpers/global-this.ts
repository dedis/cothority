
// globalThis in NodeJS appeared since 12.0.0 which means we need to declare the
// global for the LTS. As it is simply a reference to the global object (_window_
// for the browser and _global_ for NodeJS), we can assign it.
// This routine is run only for the tests as the production code should check for
// existance until we drop version above 12.0.0.
beforeAll(() => {
    // @ts-ignore
    global.globalThis = global;
});
