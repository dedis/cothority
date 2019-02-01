import Docker from 'dockerode';
import fs from 'fs';
import { Roster } from '../../src/network/proto';
import SignerEd25519 from '../../src/darc/signer-ed25519';

const docker = new Docker();
const data = fs.readFileSync(process.cwd() + '/spec/support/public.toml');

const CONTAINER_NAME = 'conode-test-run-js';
const FILTERS = JSON.stringify({ name: ['/' + CONTAINER_NAME] });

export const ROSTER = Roster.fromTOML(data);
export const BLOCK_INTERVAL = 1 * 1000 * 1000 * 1000; // 5s in nano precision
export const SIGNER = SignerEd25519.fromBytes(Buffer.from("0cb119094dbf72dfd169f8ba605069ce66a0c8ba402eb22952b544022d33b90c", "hex"));

export async function startConodes(): Promise<void> {
    const containers = await docker.listContainers({ all: true, filters: FILTERS });
    const container = containers[0];

    if (container) {
        if (container.State === 'running' || container.State === 'exited') {
            // already running
            return;
        } else {
            // clean the container to start a new one with the same name
            await docker.getContainer(container.Id).remove();
        }
    }

    const s = fs.createWriteStream('./output.log');

    docker.run('dedis/conode-test', [], s, {
        name: CONTAINER_NAME,
        Hostname: 'localhost',
        ExposedPorts: {
            '7003/tcp': {},
            '7005/tcp': {},
            '7007/tcp': {},
            '7009/tcp': {},
            '7011/tcp': {},
            '7013/tcp': {},
            '7015/tcp': {},
        },
        HostConfig: {
            PortBindings: {
                '7003/tcp': [{ HostPort: '7003' }],
                '7005/tcp': [{ HostPort: '7005' }],
                '7007/tcp': [{ HostPort: '7007' }],
                '7009/tcp': [{ HostPort: '7009' }],
                '7011/tcp': [{ HostPort: '7011' }],
                '7013/tcp': [{ HostPort: '7013' }],
                '7015/tcp': [{ HostPort: '7015' }]
            },
        },
    });

    // we can't wait for the end of the run command so we give
    // some time for the conodes to start
    await new Promise(resolve => setTimeout(resolve, 2*1000));
}

export async function stopConodes(): Promise<void> {
    const containers = await docker.listContainers({ all: true, filters: FILTERS });
    const container = containers[0];

    if (container) {
        console.log('stopping conodes...');

        // stop only the container of our tests
        await docker.getContainer(container.Id).stop();
        await docker.getContainer(container.Id).remove();
    }
}