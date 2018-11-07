package ch.epfl.dedis.byzcoin;

import ch.epfl.dedis.lib.proto.ByzCoinProto;
import com.google.protobuf.ByteString;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;

/**
 * Read-only list of state changes that also provides the hash
 * of all of the state changes
 */
public class StateChanges {
    private List<StateChange> stateChanges;
    private ByteString hash;

    /**
     * Instantiate with a list of state changes coming from a protobuf message
     * @param scs the list of state changes
     */
    public StateChanges(List<ByzCoinProto.StateChange> scs) {
        stateChanges = new ArrayList<>();

        hash(scs);
    }

    /**
     * Getter for a state change at the given index
     * @param index the index of the state change
     * @return the state change
     */
    public StateChange get(int index) {
        return stateChanges.get(index);
    }

    /**
     * Getter for the hash
     * @return the hash in byte string
     */
    public ByteString getHash() {
        return hash;
    }

    /**
     * Populate the list with the state changes and compute the hash
     * in the mean time
     * @param scs the list of state changes coming from a response
     */
    private void hash(List<ByzCoinProto.StateChange> scs) {
        MessageDigest digest;
        try {
            digest = MessageDigest.getInstance("SHA-256");
        } catch (NoSuchAlgorithmException e) {
            return;
        }

        for (ByzCoinProto.StateChange sc : scs) {
            digest.update(sc.toByteArray());
            stateChanges.add(new StateChange(sc));
        }

        hash = ByteString.copyFrom(digest.digest());
    }
}
