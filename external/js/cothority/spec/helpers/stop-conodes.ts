import { stopConodes } from "../support/conondes";

afterAll(async (done) => {
    // make sure we stop the container at the end
    await stopConodes();

    done();
});
