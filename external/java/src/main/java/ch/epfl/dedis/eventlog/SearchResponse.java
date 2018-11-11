package ch.epfl.dedis.eventlog;

import ch.epfl.dedis.lib.proto.EventLogProto;

import java.util.List;
import java.util.stream.Collectors;

/**
 * This is the response of a eventlog search request. It contains a list of events which are the events that are found
 * and a truncated flag to indicate whether the list is complete.
 */
public final class SearchResponse {

    public final List<Event> events;
    public final boolean truncated;

    /**
     * Constructs the object from a protobuf SearchResponse object.
     * @param resp The protobuf SearchResponse object.
     */
    public SearchResponse(EventLogProto.SearchResponse resp) {
        this.events = resp.getEventsList()
                .stream()
                .map(e -> new Event(e.getWhen(), e.getTopic(), e.getContent()))
                .collect(Collectors.toList());
        this.truncated = resp.getTruncated();
    }
}
