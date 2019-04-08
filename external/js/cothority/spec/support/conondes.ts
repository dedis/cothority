/* tslint:disable no-console */
import Docker from "dockerode";
import fs from "fs";
import Long from "long";
import SignerEd25519 from "../../src/darc/signer-ed25519";
import { Roster } from "../../src/network/proto";

const docker = new Docker();
const data = fs.readFileSync(process.cwd() + "/spec/support/public.toml");

const CONTAINER_NAME = "conode-test-run-js";
const FILTERS = JSON.stringify({ name: ["/" + CONTAINER_NAME] });

export const ROSTER = Roster.fromTOML(data.toString());
export const BLOCK_INTERVAL = Long.fromNumber(1 * 1000 * 1000 * 1000); // 5s in nano precision
// tslint:disable-next-line
export const SIGNER = SignerEd25519.fromBytes(Buffer.from("0cb119094dbf72dfd169f8ba605069ce66a0c8ba402eb22952b544022d33b90c", "hex"));

export async function startConodes(): Promise<void> {
    const containers = await docker.listContainers({ all: true, filters: FILTERS });
    const container = containers[0];

    if (container) {
        if (container.State === "running") {
            // already running
            return;
        } else {
            // clean the container to start a new one with the same name
            await docker.getContainer(container.Id).remove();
        }
    }

    console.log("\n=== starting conodes ===");
    console.log("Check output.log for the logs");
    const s = fs.createWriteStream("./output.log");

    docker.run("dedis/conode-test", [], s, {
        ExposedPorts: {
            "7771/tcp": {},
            "7773/tcp": {},
            "7775/tcp": {},
            "7777/tcp": {},
            "7779/tcp": {},
            "7781/tcp": {},
            "7783/tcp": {},
        },
        HostConfig: {
            PortBindings: {
                "7771/tcp": [{ HostPort: "7771" }],
                "7773/tcp": [{ HostPort: "7773" }],
                "7775/tcp": [{ HostPort: "7775" }],
                "7777/tcp": [{ HostPort: "7777" }],
                "7779/tcp": [{ HostPort: "7779" }],
                "7781/tcp": [{ HostPort: "7781" }],
                "7783/tcp": [{ HostPort: "7783" }],
            },
        },
        Hostname: "localhost",
        name: CONTAINER_NAME,
    });

    // we can't wait for the end of the run command so we give
    // some time for the conodes to start
    await new Promise((resolve) => setTimeout(resolve, 10 * 1000));

    console.log("=== conodes started ===");
}

export async function stopConodes(): Promise<void> {
    const containers = await docker.listContainers({ all: true, filters: FILTERS });
    const container = containers[0];

    if (container) {
        console.log("\n\n=== stopping conodes ===\n");

        // stop only the container of our tests
        await docker.getContainer(container.Id).stop();
        await docker.getContainer(container.Id).remove();
    }

    if (process.env.CI) {
        // Write the logs for continuous integration as we don't have access
        // to the file

        await new Promise((resolve) => {
            const logs = fs.createReadStream("./output.log");

            logs.on("data", (chunk) => {
                if (chunk instanceof Buffer) {
                    console.log(chunk.toString());
                } else {
                    console.log(chunk);
                }
            });

            logs.on("close", resolve);
        });
    }
}
