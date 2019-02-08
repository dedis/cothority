package ch.epfl.dedis.integration;

import ch.epfl.dedis.lib.network.ServerIdentity;
import com.github.dockerjava.api.DockerClient;
import com.github.dockerjava.api.command.ExecCreateCmdResponse;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.testcontainers.containers.Container;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.output.FrameConsumerResultCallback;
import org.testcontainers.containers.output.OutputFrame;
import org.testcontainers.containers.output.Slf4jLogConsumer;
import org.testcontainers.containers.wait.strategy.Wait;
import org.testcontainers.images.builder.ImageFromDockerfile;

import java.io.IOException;
import java.time.LocalDateTime;
import java.util.Arrays;
import java.util.List;

public class DockerTestServerController extends TestServerController {
    private static final Logger logger = LoggerFactory.getLogger(DockerTestServerController.class);
    private static final String TEST_SERVER_IMAGE_NAME = "dedis/conode-test:latest";
    private static final String TEMPORARY_DOCKER_IMAGE = "conode-test-run";

    private final GenericContainer<?> blockchainContainer;

    DockerTestServerController() {
        super();
        logger.warn("local docker will be started for tests.");
        logger.info("This test run assumes that image " + TEST_SERVER_IMAGE_NAME + " is available in your system.");
        logger.info("To build such image you should run `make docker docker_test` - such run will create base image and image with test keys.");
        logger.info("For a test run this code will create additional docker image with name " + TEMPORARY_DOCKER_IMAGE +
                ", at the end this additional image will be automatically deleted");
        try {
            blockchainContainer = new GenericContainer<>(
                    new ImageFromDockerfile(TEMPORARY_DOCKER_IMAGE, true)
                            .withDockerfileFromBuilder(builder -> builder
                                    .from(TEST_SERVER_IMAGE_NAME)
                                    .expose(7770, 7771, 7772, 7773, 7774, 7775, 7776, 7777, 7778, 7779, 7780, 7781, 7782, 7783))
            );

            blockchainContainer.setPortBindings(Arrays.asList(
                    "7770:7770", "7771:7771",
                    "7772:7772", "7773:7773",
                    "7774:7774", "7775:7775",
                    "7776:7776", "7777:7777",
                    "7778:7778", "7779:7779",
                    "7780:7780", "7781:7781",
                    "7782:7782", "7783:7783"));
            blockchainContainer.withExposedPorts(7770, 7771, 7772, 7773, 7774, 7775, 7776, 7777);
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
    public void startConode(int nodeNumber) throws InterruptedException {
        if (nodeNumber <= 0) {
            throw new InterruptedException("Node numbering starts at 1!");
        }
        logger.info("Starting container co{}/private.toml", nodeNumber);
        runCmdInBackgroundStd(blockchainContainer, "env", "COTHORITY_ALLOW_INSECURE_ADMIN=1", "DEBUG_TIME=true", "CONODE_SERVICE_PATH=.",
                "conode", "-d", "2", "-c", "co" + nodeNumber + "/private.toml", "server");
        // Wait a bit for the server to actually start.
        Thread.sleep(1000);
    }

    @Override
    public void killConode(int nodeNumber) throws IOException, InterruptedException {
        if (nodeNumber <= 0) {
            throw new InterruptedException("Node numbering starts at 1!");
        }
        logger.info("Killing container co{}/private.toml", nodeNumber);
        Container.ExecResult psResults = blockchainContainer.execInContainer("ps", "-o", "pid=,command=", "-C", "conode");
        for (String psLine : psResults.getStdout().split("\\n")) {
            if (psLine.contains("co" + nodeNumber + "/private.toml")) {
                String pid = psLine.trim().split("\\s")[0];
                blockchainContainer.execInContainer("kill", pid);
                break;
            }
        }
    }

    /**
     * We only get 4 conodes because the run_conode.sh file (from the Dockerfile) only starts 4 conodes.
     * The other conodes (5 to 7) are used for testing roster changes.
     */
    @Override
    public List<ServerIdentity> getConodes() {
        return getIdentities().subList(0, 4);
    }

    private void runCmdInBackground(GenericContainer container, String... cmd) throws InterruptedException {
        DockerClient dockerClient = container.getDockerClient();

        ExecCreateCmdResponse execCreateCmdResponse = dockerClient.execCreateCmd(container.getContainerId())
                .withAttachStdout(true)
                .withAttachStderr(true)
                .withAttachStdin(false)
                .withCmd(cmd)
                .exec();

        FrameConsumerResultCallback fc = new FrameConsumerResultCallback();
        Slf4jLogConsumer logConsumer = new Slf4jLogConsumer(logger);
        fc.addConsumer(OutputFrame.OutputType.STDOUT, logConsumer);
        fc.addConsumer(OutputFrame.OutputType.STDERR, logConsumer);

        dockerClient.execStartCmd(execCreateCmdResponse.getId())
                .exec(fc).awaitStarted();
    }

    private void runCmdInBackgroundStd(GenericContainer container, String... cmd) throws InterruptedException {
        DockerClient dockerClient = container.getDockerClient();

        ExecCreateCmdResponse execCreateCmdResponse = dockerClient.execCreateCmd(container.getContainerId())
                .withAttachStdout(true)
                .withAttachStderr(true)
                .withAttachStdin(false)
                .withCmd(cmd)
                .exec();

        FrameConsumerResultCallback fc = new FrameConsumerResultCallback();
        Slf4jLogConsumer logConsumer = new Slf4jLogConsumer(logger);
        fc.addConsumer(OutputFrame.OutputType.STDOUT, logConsumer);
        fc.addConsumer(OutputFrame.OutputType.STDERR, logConsumer);

        dockerClient.execStartCmd(execCreateCmdResponse.getId())
                .exec(fc).awaitStarted();
    }
}
