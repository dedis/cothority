package ch.epfl.dedis.ocs;

import ch.epfl.dedis.lib.Crypto;

/**
 * dedis/lib
 * Account.java
 * Purpose: Represents one account on the client side. This class will wrap
 * whatever needs to be done for a smartcard or other authentication.
 *
 * @author Linus Gasser <linus.gasser@epfl.ch>
 * @version 0.2 17/09/19
 */

public class Account {
    static public int ADMIN = 1;
    static public int WRITER = 2;
    static public int READER = 4;

    // ID must be unique and identifies an account.
    public byte[] ID;
    public Crypto.Point Point;
    public Crypto.Scalar Scalar;
    public int Access;

    public Account(int a) throws Exception{
        Access = a;
        ID = Crypto.uuid4();

        Crypto.KeyPair kp = new Crypto.KeyPair();
        Scalar = kp.Scalar;
        Point = kp.Point;
    }
}

