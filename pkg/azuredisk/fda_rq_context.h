#ifndef FDA_RQ_CTX_H
#define FDA_RQ_CTX_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include<sys/ioctl.h>
#include <errno.h>
#include <time.h>

#include <stdint.h>

#define IOCTL_XS_FASTPATH_DRIVER_USER_REQUEST _IOWR('R', 10, struct xs_fastpath_request_sync)

#define MAX_RESPONSE_SIZE  (32 * 1024)
#define MAX_REQUEST_SIZE  (8 * 1024)

struct xs_fastpath_request_sync_response {
        int32_t status;
        int32_t response_len;
};

struct xs_fastpath_request_sync {
        char guid[16];
        int32_t timeout;
        int32_t request_len;
        int32_t response_len;
        int32_t data_len;
        int32_t data_valid;
        int64_t request_buffer;
        int64_t response_buffer;
        int64_t data_buffer;
        struct xs_fastpath_request_sync_response response;
};

struct qad_request_buffer {
	int32_t op_id; // op_id = 1 for attach, op_id = 2 for detach operation
	int32_t lun_num;
	char blobstr[MAX_REQUEST_SIZE-8];
};

struct qad_data_buffer {
	int32_t data_val;
	int32_t dummy;
	char data_str[MAX_RESPONSE_SIZE-8];
};
struct qad_response_buffer {
	int32_t response_val;
	int32_t dummy;
	char response_str[MAX_RESPONSE_SIZE-8];
};

typedef struct _fda_vsc_context_t {
    int32_t fvc_fd;
    uint32_t fvc_attach_cnt;
    uint32_t fvc_detach_cnt;
} fda_vsc_context_t;

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

#endif /* FDA_RQ_CTX */
