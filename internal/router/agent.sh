#!/bin/sh
# conntrack-agent: lightweight outbound connection monitor for OpenWrt
# Streams new outbound TCP/UDP connections to an opensnitch-web server.
# Designed for busybox ash — no bash-isms.

CONFIG="/etc/conntrack-agent/config"
if [ ! -f "$CONFIG" ]; then
    logger -t conntrack-agent "config not found: $CONFIG"
    exit 1
fi
. "$CONFIG"

BATCH_FILE="/tmp/conntrack-agent-batch"
MAX_BATCH=${BATCH_SIZE:-20}
FLUSH_INTERVAL=${BATCH_INTERVAL:-5}

# Ensure clean start
rm -f "$BATCH_FILE"
echo '{"events":[' > "$BATCH_FILE"

is_private_ip() {
    case "$1" in
        10.*) return 0 ;;
        172.1[6-9].*|172.2[0-9].*|172.3[0-1].*) return 0 ;;
        192.168.*) return 0 ;;
        127.*) return 0 ;;
        224.*|239.*) return 0 ;;
        255.255.255.255) return 0 ;;
        *) return 1 ;;
    esac
}

is_lan_ip() {
    case "$1" in
        ${LAN_PREFIX}*) return 0 ;;
        *) return 1 ;;
    esac
}

flush_batch() {
    LINES=$(wc -l < "$BATCH_FILE" 2>/dev/null || echo 0)
    if [ "$LINES" -le 1 ]; then
        # No events — send empty heartbeat so the server keeps us online
        PAYLOAD='{"events":[]}'
    else
        # Close the JSON array (remove trailing comma via sed)
        sed -i '$ s/,$//' "$BATCH_FILE"
        echo ']}' >> "$BATCH_FILE"
        PAYLOAD=$(cat "$BATCH_FILE")
    fi

    wget -q -O /dev/null \
        --post-data="$PAYLOAD" \
        --header="X-API-Key: $API_KEY" \
        --header="Content-Type: application/json" \
        --timeout=10 \
        "$SERVER_URL/api/v1/ingest" 2>/dev/null

    if [ $? -ne 0 ]; then
        logger -t conntrack-agent "POST failed, dropped batch"
    fi

    # Reset
    rm -f "$BATCH_FILE"
    echo '{"events":[' > "$BATCH_FILE"
}

trap 'flush_batch; exit 0' INT TERM

# Main loop: poll conntrack table periodically instead of event streaming.
# This avoids signal delivery issues with ash pipelines and is more
# reliable on resource-constrained routers.
PREV_SEEN_FILE="/tmp/conntrack-agent-seen.prev"
CURR_SEEN_FILE="/tmp/conntrack-agent-seen.curr"
rm -f "$PREV_SEEN_FILE" "$CURR_SEEN_FILE"
touch "$PREV_SEEN_FILE"

while true; do
    : > "$CURR_SEEN_FILE"

    # Snapshot current connections — conntrack -L outputs one line per entry:
    # tcp  6 117 SYN_SENT src=192.168.1.10 dst=8.8.8.8 sport=54321 dport=53 ...
    conntrack -L 2>/dev/null | while IFS= read -r LINE; do
        # Extract protocol: first word on the line
        PROTO=$(echo "$LINE" | awk '{print $1}')
        SRC=""
        DST=""
        SPORT=""
        DPORT=""

        # Parse key=value fields
        for FIELD in $LINE; do
            case "$FIELD" in
                src=*)
                    VAL="${FIELD#src=}"
                    [ -z "$SRC" ] && SRC="$VAL"
                    ;;
                dst=*)
                    VAL="${FIELD#dst=}"
                    [ -z "$DST" ] && DST="$VAL"
                    ;;
                sport=*)
                    VAL="${FIELD#sport=}"
                    [ -z "$SPORT" ] && SPORT="$VAL"
                    ;;
                dport=*)
                    VAL="${FIELD#dport=}"
                    [ -z "$DPORT" ] && DPORT="$VAL"
                    ;;
            esac
        done

        # Skip incomplete
        [ -z "$PROTO" ] || [ -z "$SRC" ] || [ -z "$DST" ] || [ -z "$DPORT" ] && continue

        # Filter: only LAN src -> non-private dst (outbound to internet)
        is_lan_ip "$SRC" || continue
        is_private_ip "$DST" && continue

        [ -z "$SPORT" ] && SPORT="0"

        FLOW_KEY="${PROTO}_${SRC}_${SPORT}_${DST}_${DPORT}"
        echo "$FLOW_KEY" >> "$CURR_SEEN_FILE"
        grep -qF "$FLOW_KEY" "$PREV_SEEN_FILE" && continue

        printf '{"protocol":"%s","src_ip":"%s","src_port":%s,"dst_ip":"%s","dst_host":"","dst_port":%s},\n' \
            "$PROTO" "$SRC" "$SPORT" "$DST" "$DPORT" >> "$BATCH_FILE"

    done

    # Flush after each poll cycle
    flush_batch

    mv "$CURR_SEEN_FILE" "$PREV_SEEN_FILE"

    sleep "$FLUSH_INTERVAL"
done
