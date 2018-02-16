package ch.epfl.dedis.integration;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class TestServerInit {
    private static final Logger logger = LoggerFactory.getLogger(TestServerInit.class);
    private TestServerController controllerImplementation;
    private static String testWithoutDocker = System.getenv("TEST_WITHOUT_DOCKER");

    private static class Holder {
        private static final TestServerController INSTANCE = new TestServerInit().getControllerImplementation();
    }

    public static TestServerController getInstance() {
        return Holder.INSTANCE;
    }

    public static TestServerController getInstanceManual() {
        testWithoutDocker = "yes";
        return Holder.INSTANCE;
    }

    private TestServerInit() {
        try {

            if (testWithoutDocker != null) {
                logger.info("tests will not start docker with conodes for you. Remember to do it by your self");
                controllerImplementation = new ManualTestServerController();
            }
            else {
                logger.info("starting docker for tests");
                controllerImplementation = new DockerTestServerController();
            }
        } catch (Exception e) {
            throw new IllegalStateException(e);
        }
    }

    private TestServerController getControllerImplementation() {
        return controllerImplementation;
    }
}
