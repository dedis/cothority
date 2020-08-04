package ch.epfl.dedis.lib;

public interface HashId {
    /**
     * Return binary form of block getId used by skipchain
     * @return binary form of block getId
     */
    byte [] getId();
}
