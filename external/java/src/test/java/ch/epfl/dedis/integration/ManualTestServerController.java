package ch.epfl.dedis.integration;

import ch.epfl.dedis.lib.network.ServerIdentity;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.util.List;
import java.util.stream.Collectors;


public class ManualTestServerController extends TestServerController {
    private static final Logger logger = LoggerFactory.getLogger(ManualTestServerController.class);

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
    public void cleanDBs(){
        logger.info("Not cleaning DBs in manual mode");
    }

    @Override
    public List<ServerIdentity> getConodes() {
        return getIdentities().subList(0, 4);
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
