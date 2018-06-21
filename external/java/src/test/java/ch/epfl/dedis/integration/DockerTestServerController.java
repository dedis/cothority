package ch.epfl.dedis.integration;

import ch.epfl.dedis.byzgen.OcsFactory;
import com.github.dockerjava.api.DockerClient;
import com.github.dockerjava.api.command.ExecCreateCmdResponse;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.testcontainers.containers.Container;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.output.FrameConsumerResultCallback;
import org.testcontainers.containers.output.Slf4jLogConsumer;
import org.testcontainers.containers.wait.Wait;
import org.testcontainers.images.builder.ImageFromDockerfile;

import java.io.IOException;
import java.time.LocalDateTime;
import java.util.Arrays;
import java.util.List;

public class DockerTestServerController extends TestServerController {
    private static final Logger logger = LoggerFactory.getLogger(DockerTestServerController.class);
    private static final String TEST_SERVER_IMAGE_NAME = "dedis/conode-test:latest";
    private static final String TEMPORARY_DOCKER_IMAGE = "conode-test-run";

    private final GenericContainer blockchainContainer;

    protected DockerTestServerController() {
        logger.warn("local docker will be started for tests.");
        logger.info("This test run assumes that image " + TEST_SERVER_IMAGE_NAME + " is available in your system.");
        logger.info("To build such image you should run `make docker docker_test` - such run will create base image and image with test keys.");
        logger.info("For a test run this code will create additional docker image with name " + TEMPORARY_DOCKER_IMAGE +
                ", at the end this additional image will be automatically deleted");
        try {
            blockchainContainer = new GenericContainer(
                    new ImageFromDockerfile(TEMPORARY_DOCKER_IMAGE, true)
                            .withDockerfileFromBuilder(builder -> {
                                builder
                                        .from(TEST_SERVER_IMAGE_NAME)
                                        .expose(7002, 7003, 7004, 7005, 7006, 7007, 7008, 7009);
                            })
            );

            blockchainContainer.setPortBindings(Arrays.asList(
                    "7002:7002", "7003:7003",
                    "7004:7004", "7005:7005",
                    "7006:7006", "7007:7007",
                    "7008:7008", "7009:7009"));
            blockchainContainer.withExposedPorts(7002, 7003, 7004, 7005, 7006, 7007, 7008, 7009);
            blockchainContainer.withExtraHost("conode1", "127.0.0.1");
            blockchainContainer.withExtraHost("conode2", "127.0.0.1");
            blockchainContainer.withExtraHost("conode3", "127.0.0.1");
            blockchainContainer.withExtraHost("conode4", "127.0.0.1");
            blockchainContainer.waitingFor(Wait.forListeningPort());
            blockchainContainer.start();
            Slf4jLogConsumer logConsumer = new Slf4jLogConsumer(logger);
            blockchainContainer.withLogConsumer(logConsumer);
            blockchainContainer.followOutput(logConsumer);
            logger.info("Started at {}", LocalDateTime.now());
        } catch (Exception e) {
            logger.info("Exception at {}", LocalDateTime.now());
            throw new IllegalStateException("Cannot start docker image with test server. Please ensure that local conodes are not running.", e);
        }
    }

    @Override
    public int countRunningConodes() throws IOException, InterruptedException {
        Container.ExecResult psResults = blockchainContainer.execInContainer("ps", "-o", "pid=", "-C", "conode");
        return psResults.getStdout().split("\\n").length;
    }

    @Override
    public void startConode(int nodeNumber) throws InterruptedException {
        runCmdInBackground(blockchainContainer, "conode", "-d", "2", "-c", "co" + nodeNumber + "/private.toml", "server");
    }

    @Override
    public void killConode(int nodeNumber) throws IOException, InterruptedException {
        Container.ExecResult psResults = blockchainContainer.execInContainer("ps", "-o", "pid=,command=", "-C", "conode");
        for (String psLine :psResults.getStdout().split("\\n") ) {
            if (psLine.contains("co" + nodeNumber + "/private.toml")) {
                String pid = psLine.trim().split("\\s")[0];
                blockchainContainer.execInContainer("kill", pid);
                break;
            }
        }
    }

    @Override
    public List<OcsFactory.ConodeAddress> getConodes() {
        return Arrays.asList(
                new OcsFactory.ConodeAddress(buildURI("tcp://" + blockchainContainer.getContainerIpAddress() + ":7002"), CONODE_PUB_1),
                new OcsFactory.ConodeAddress(buildURI("tcp://localhost:7004"), CONODE_PUB_2),
                new OcsFactory.ConodeAddress(buildURI("tcp://localhost:7006"), CONODE_PUB_3),
                new OcsFactory.ConodeAddress(buildURI("tcp://localhost:7008"), CONODE_PUB_4));
    }

    private void runCmdInBackground(GenericContainer container, String ... cmd) throws InterruptedException {
        DockerClient dockerClient = container.getDockerClient();

        ExecCreateCmdResponse execCreateCmdResponse = dockerClient.execCreateCmd(container.getContainerId())
                .withAttachStdout(false)
                .withAttachStderr(false)
                .withAttachStdin(false)
                .withCmd(cmd)
                .exec();

        dockerClient.execStartCmd(execCreateCmdResponse.getId())
                .exec(new FrameConsumerResultCallback()).awaitStarted();
    }
}
