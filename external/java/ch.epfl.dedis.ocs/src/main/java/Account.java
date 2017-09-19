public class Account {
    static public int ADMIN = 1;
    static public int WRITER = 2;
    static public int READER = 4;

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

