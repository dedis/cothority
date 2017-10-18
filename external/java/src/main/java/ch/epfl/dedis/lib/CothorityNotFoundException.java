package ch.epfl.dedis.lib;

public class CothorityNotFoundException extends CothorityException {
    public CothorityNotFoundException() {
    }

    public CothorityNotFoundException(String message) {
        super(message);
    }

    public CothorityNotFoundException(String message, Throwable cause) {
        super(message, cause);
    }

    public CothorityNotFoundException(Throwable cause) {
        super(cause);
    }

    public CothorityNotFoundException(String message, Throwable cause, boolean enableSuppression, boolean writableStackTrace) {
        super(message, cause, enableSuppression, writableStackTrace);
    }
}
