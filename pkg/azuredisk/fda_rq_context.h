#ifndef FDA_RQ_CTX_H
#define FDA_RQ_CTX_H

#include <stdint.h>

#define FDA_RQ_MAGIC 0xCAFEF00D
#define FDA_RQ_VERSION 10

typedef struct _fda_vsc_context_t {
    int32_t fvc_fd;
    uint32_t fvc_attach_cnt;
    uint32_t fvc_detach_cnt;
} fda_vsc_context_t;

typedef enum {
    FDA_INVALID_REQUEST_STATE = 0,
    FDA_REQ_INITED,
    FDA_REQ_INIT_FAILED,
    FDA_REQ_ISSUED,
    FDA_REQ_RECD,
    FDA_REQ_PROCESSING,
    FDA_REQ_PROCESSED,
    FDA_REQ_RETURNED,
    FDA_MAX_REQUEST_STATE,
} fda_request_state_t;

typedef enum {
    FDA_INVALID_OP = 0,
    FDA_ATTACH,
    FDA_DETACH,
    FDA_MAX_OP,
} fda_op_t;

typedef enum {
    FDA_INVALID_STATUS = 0,
    FDA_OP_SUCCEEDED,
    FDA_OP_FAILED,
    /* other status? */
    FDA_MAX_STATUS,
} fda_op_status_t;

/*
 * Note: If the activity id, vmid, bloburl, and dsas key are guaranteed to be of
 * a fixed length, then we could just store them as appropriately-sized struct
 * fields directly.
 *
 * In the absence of such information, we dump said fields into a bag-of-bytes
 * as below.
 */
typedef struct _fda_request_t {
    uint32_t fbi_magic;
    uint32_t fbi_version;
    uint32_t fbi_length;
    uint32_t fbi_pad;
    fda_op_t fbi_rq_state;
    fda_op_t fbi_op;
    fda_op_status_t fbi_op_status;
    uint32_t fbi_pad1;
    uint32_t fbi_activity_id_offset;
    uint32_t fbi_activity_id_length;
    uint32_t fbi_vmid_offset;
    uint32_t fbi_vmid_length;
    uint32_t fbi_data_bloburl_offset;
    uint32_t fbi_data_bloburl_length;
    uint32_t fbi_data_dsas_key_offset;
    uint32_t fbi_data_dsas_key_length;
    uint32_t fbi_data[];
} fda_request_t;

#endif /* FDA_RQ_CTX */
