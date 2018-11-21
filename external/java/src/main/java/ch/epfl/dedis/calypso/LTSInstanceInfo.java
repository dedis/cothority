package ch.epfl.dedis.calypso;

import ch.epfl.dedis.lib.Roster;
import ch.epfl.dedis.lib.exception.CothorityException;
import ch.epfl.dedis.lib.proto.Calypso;

import java.net.URISyntaxException;

public class LTSInstanceInfo {
    private Roster roster;
    public LTSInstanceInfo(Calypso.LtsInstanceInfo proto) throws CothorityException {
        try {
            this.roster = new Roster(proto.getRoster());
        } catch (URISyntaxException e) {
            throw new CothorityException(e.getMessage());
        }
    }

    public LTSInstanceInfo(Roster roster) {
        this.roster = roster;
    }

    public Calypso.LtsInstanceInfo toProto() {
        Calypso.LtsInstanceInfo.Builder b = Calypso.LtsInstanceInfo.newBuilder();
        b.setRoster(this.roster.toProto());
        return b.build();
    }

    public Roster getRoster() {
        return roster;
    }
}
