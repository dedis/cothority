package ch.epfl.dedis.lib;

public class CothorityPermissionException extends CothorityException {
    public CothorityPermissionException() {
    }

    public CothorityPermissionException(String message) {
        super(message);
    }

    public CothorityPermissionException(String message, Throwable cause) {
        super(message, cause);
    }

    public CothorityPermissionException(Throwable cause) {
        super(cause);
    }

    public CothorityPermissionException(String message, Throwable cause, boolean enableSuppression, boolean writableStackTrace) {
        super(message, cause, enableSuppression, writableStackTrace);
    }
}
