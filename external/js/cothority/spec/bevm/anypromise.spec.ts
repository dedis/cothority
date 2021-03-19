// For debugging
// import Log from "../../src/log";

import { BEvmRPC } from "../../src/bevm";

describe("anyPromise", () => {
  function delay(duration: number) {
      return new Promise(
          (resolve: (val: number) => void) => setTimeout(resolve, duration, duration));
  }

  function fail(duration: number) {
      return new Promise(
          (_, reject: (val: number) => void) => setTimeout(reject, duration, duration));
  }

  it("should be rejected when providing no promise", async () => {
    await expectAsync(BEvmRPC.anyPromise([])).toBeRejected();
  });

  it("should be resolved with the first resolved promise", async () => {
    await expectAsync(BEvmRPC.anyPromise([
        delay(10), delay(20), delay(30),
    ])).toBeResolvedTo(10);

    await expectAsync(BEvmRPC.anyPromise([
        delay(20), delay(10), delay(30),
    ])).toBeResolvedTo(10);

    await expectAsync(BEvmRPC.anyPromise([
        delay(20), delay(30), delay(10),
    ])).toBeResolvedTo(10);

    await expectAsync(BEvmRPC.anyPromise([
        fail(10), delay(20), delay(30),
    ])).toBeResolvedTo(20);

    await expectAsync(BEvmRPC.anyPromise([
        delay(10), fail(20), delay(30),
    ])).toBeResolvedTo(10);

    await expectAsync(BEvmRPC.anyPromise([
        delay(10), delay(20), fail(30),
    ])).toBeResolvedTo(10);

    await expectAsync(BEvmRPC.anyPromise([
        fail(10), fail(20), delay(30),
    ])).toBeResolvedTo(30);

    await expectAsync(BEvmRPC.anyPromise([
        fail(10), delay(20), fail(30),
    ])).toBeResolvedTo(20);

    await expectAsync(BEvmRPC.anyPromise([
        delay(10), fail(20), fail(30),
    ])).toBeResolvedTo(10);
  });

  it("should be rejected when all promises are rejected", async () => {
    await expectAsync(BEvmRPC.anyPromise([
        fail(10), fail(20), fail(30),
    ])).toBeRejectedWith([10, 20, 30]);

    // The order of errors is preserved
    await expectAsync(BEvmRPC.anyPromise([
        fail(30), fail(20), fail(10),
    ])).toBeRejectedWith([30, 20, 10]);
  });
});
