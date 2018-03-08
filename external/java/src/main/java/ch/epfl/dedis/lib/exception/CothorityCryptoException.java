package ch.epfl.dedis.lib.exception;

public class CothorityCryptoException extends CothorityException{
    public CothorityCryptoException(String m) {
        super(m);
    }

    public CothorityCryptoException(String message, Throwable cause) {
        super(message, cause);
    }
}
