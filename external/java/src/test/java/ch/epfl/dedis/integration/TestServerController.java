package ch.epfl.dedis.integration;

import java.io.IOException;

public interface TestServerController {
    int countRunningConodes() throws IOException, InterruptedException;

    void startConode(int nodeNumber) throws InterruptedException, IOException;

    void killConode(int nodeNumber) throws IOException, InterruptedException;
}
