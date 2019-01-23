import Docker from 'dockerode';

const docker = new Docker();

beforeAll(async function (done) {
    console.log('starting conodes...');

    docker.run('dedis/conode-test', [], process.stdout, {
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
    setTimeout(done, 2 * 1000);
}, 30 * 1000);

afterAll(async function () {
    console.log('stopping conodes...');

    const containers = await docker.listContainers();

    await Promise.all(containers.map(({ Id }) => docker.getContainer(Id).stop()));
}, 30 * 1000);
