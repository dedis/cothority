package ch.epfl.dedis.lib.exception;

public class CothorityException extends Exception {
    public CothorityException() {
    }

    public CothorityException(String message) {
        super(message);
    }

    public CothorityException(String message, Throwable cause) {
        super(message, cause);
    }

    public CothorityException(Throwable cause) {
        super(cause);
    }

    public CothorityException(String message, Throwable cause, boolean enableSuppression, boolean writableStackTrace) {
        super(message, cause, enableSuppression, writableStackTrace);
    }
}
