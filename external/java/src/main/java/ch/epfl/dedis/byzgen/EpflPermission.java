package ch.epfl.dedis.byzgen;

import java.util.Collections;
import java.util.EnumSet;
import java.util.Set;

/**
 * Global system permissions.
 */
public enum EpflPermission {
    ADMIN(1), WRITER(2), READER(4);

    private final int bitRepresentation;

    EpflPermission(int bitRepresentation) {
        this.bitRepresentation = bitRepresentation;
    }

    int getBitRepresentation() {
        return bitRepresentation;
    }

    public static EnumSet<EpflPermission> setOf(EpflPermission ... permissions) {
        EnumSet<EpflPermission> enumSet = EnumSet.noneOf(EpflPermission.class);
        Collections.addAll(enumSet, permissions);
        return enumSet;
    }

    public static int maskOf(Set<EpflPermission> permissionSet) {
        int mask = 0;
        for (EpflPermission permission : permissionSet) {
            mask |= permission.getBitRepresentation();
        }
        return mask;
    }

    public static EnumSet<EpflPermission> setOf(int bitmask) {
        EnumSet<EpflPermission> enumSet = EnumSet.noneOf(EpflPermission.class);

        for (EpflPermission permission : EpflPermission.values()) {
            if (0 != (bitmask & permission.getBitRepresentation())) {
                enumSet.add(permission);
            }
        }
        return enumSet;
    }
}