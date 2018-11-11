package ch.epfl.dedis.integration;

import ch.epfl.dedis.byzgen.CalypsoFactory;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;


public class ManualTestServerController extends TestServerController {
    @Override
    public void startConode(int nodeNumber) throws InterruptedException, IOException {
        Runtime.getRuntime().exec("../scripts/start_4th_conode.sh");
        Thread.sleep(1000);
    }

    @Override
    public void killConode(int nodeNumber) throws IOException, InterruptedException {
        if (nodeNumber!=4) {
            throw new IllegalArgumentException("I'm a manual controller and I'm able only to kill node4");
        }

        // kill the last conode and try to make a request
        int exitValue = Runtime.getRuntime().exec("pkill -n conode").waitFor();

        if ( exitValue != 0 ) {
            throw new IllegalStateException("something is wrong I'm not able to kill node");
        }
    }

    @Override
    public List<CalypsoFactory.ConodeAddress> getConodes() {
        return Arrays.asList(
                new CalypsoFactory.ConodeAddress(buildURI("tls://localhost:7002"), CONODE_PUB_1),
                new CalypsoFactory.ConodeAddress(buildURI("tls://localhost:7004"), CONODE_PUB_2),
                new CalypsoFactory.ConodeAddress(buildURI("tls://localhost:7006"), CONODE_PUB_3),
                new CalypsoFactory.ConodeAddress(buildURI("tls://localhost:7008"), CONODE_PUB_4));
    }

    private static int countLines(String str){
        String[] lines = str.split("\r\n|\r|\n");
        return  lines.length;
    }

    private static String inputStreamToString(InputStream in) {
        return new BufferedReader(new InputStreamReader(in))
                .lines().collect(Collectors.joining("\n"));
    }
}
