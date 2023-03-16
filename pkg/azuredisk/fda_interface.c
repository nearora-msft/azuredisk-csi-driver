#include "fda_interface.h"

/* Nothing fancy for PoC purposes; go with malloc(3). Consider a dedicated
 * allocator in prod.
 */
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>

#include <errno.h>
#include <time.h>

fda_vsc_context_t *fda_vsc_init()
{
    fda_vsc_context_t *ctx = NULL;

    ctx = malloc(sizeof(fda_vsc_context_t));
    if (ctx == NULL) {
        goto out;
    }

    ctx->fvc_fd = -1;
    ctx->fvc_attach_cnt = 0;
    ctx->fvc_detach_cnt = 0;

    printf("\nOpening VSC Driver\n");
    ctx->fvc_fd = open("/dev/azure_blob", O_RDWR);
    if (ctx->fvc_fd < 0) {
        perror("VSC driver opening failed!");
        goto err_out_free;
    }

out:
    return ctx;

err_out_free:
    free(ctx);
    ctx = NULL;
    return ctx;
}

int fda_disk_attach(fda_vsc_context_t *ctx,
    const char *fda_bloburl,
    uint32_t fda_bloburl_length,
    uint32_t fda_lun_number)
{
    int status = FDA_INVALID_STATUS;

	struct xs_fastpath_request_sync request;
	struct qad_request_buffer req_buffer;
	struct qad_data_buffer data_buffer;
	struct qad_response_buffer response_buffer;
	int request_len = MAX_REQUEST_SIZE, data_len = MAX_RESPONSE_SIZE, response_len = MAX_RESPONSE_SIZE;
	int count;
	int bloblen = 0;
	int ioctl_response = 0;

    if (ctx == NULL) {
        goto out;
    }

	memset(&request, 0, sizeof(request));
	for (count=0; count<16; count++)
		request.guid[count] = count;
	request.timeout = 10000; // 10 sec timeout

	// initialize buffer with 0 values
	request.request_len = request_len;
	request.request_buffer = malloc(request.request_len);
	memset(request.request_buffer, 0x0, request.request_len);

	request.response_len = response_len;
	request.response_buffer = malloc(request.response_len);
	memset(request.response_buffer, 0x0, request.response_len);

	request.data_len = data_len;
        request.data_valid = 1;
	request.data_buffer = malloc(request.data_len);
	memset(request.data_buffer, 0x0, request.data_len);

	//sample blob format
	//"%NODE%/%ACCOUNT%/%CONTAINER%/%BLOBNAME%?sr=b&sk=system-1&sv=2014-02-14&sp=rw&se=9999-01-01,%DSASKEY%,0,%DOMAIN%"
	//"XDISK:0.0.0.0/md-pbkwsw54zpvl/xppxscqhzxzg/abcd?sr=b&sp=rw&se=9999-01-01&sk=system-1&sv=2014-02-14,$rzSYNQOjz4Rjb5tFVk4O0JQuK2CeKCW/R4kfiGDWfjc=$,0,z44.blob.storage.azure.net"

	bloblen = strlen(fda_bloburl);
	if(bloblen != fda_bloburl_length)
	{
		printf("blobstr length mismatch.");
		return FDA_INVALID_STATUS;
	}

	printf("Bloburl length = %d\n", bloblen);
	printf("Bloburl with DSAS key = %s\n", fda_bloburl);

	// Init Request Buffer
	// op_id = 1 for attach, op_id = 2 for detach operation
	req_buffer.op_id = FDA_ATTACH; // 1 for attach op
	req_buffer.lun_num = fda_lun_number;
	memcpy(req_buffer.blobstr, fda_bloburl, strlen(fda_bloburl));
	memcpy(request.request_buffer, &req_buffer, bloblen+8);

    struct timespec ts1, ts2; // time latency calculation
	clock_gettime(CLOCK_MONOTONIC, &ts1);

	printf("Attach Op: Sending IOCTL to VSC driver\n");
	ioctl_response = ioctl(ctx->fvc_fd, IOCTL_XS_FASTPATH_DRIVER_USER_REQUEST, &request);
    printf("IOCTL response = %d errno %d\n", ioctl_response, errno);

	clock_gettime(CLOCK_MONOTONIC, &ts2);
    double qadLatencyMs = (1000.0*ts2.tv_sec + 1e-6*ts2.tv_nsec)
                          - (1000.0*ts1.tv_sec + 1e-6*ts1.tv_nsec);

	if (ioctl_response == 0)
	{
	    printf("IOCTL passed.\n");
		printf("QAD latency for IOCTL_XS_FASTPATH_DRIVER_USER_REQUEST response: %.3f ms.\n", qadLatencyMs);
		status = FDA_OP_SUCCEEDED;
	}
	else
	{
		perror("IOCTL failed.");
		printf("IOCTL failed: %Return value status %0X response_len %u\n", request.response.status, request.response.response_len);
		status = FDA_OP_FAILED;
	}

out:
    return status;
}

int fda_disk_detach(fda_vsc_context_t *ctx,
    const char *fda_bloburl,
    uint32_t fda_bloburl_length,
    uint32_t fda_lun_number)
{
    int status = FDA_INVALID_STATUS;

	struct xs_fastpath_request_sync request;
	struct qad_request_buffer req_buffer;
	struct qad_data_buffer data_buffer;
	struct qad_response_buffer response_buffer;
	int request_len = MAX_REQUEST_SIZE, data_len = MAX_RESPONSE_SIZE, response_len = MAX_RESPONSE_SIZE;
	int count;
	int bloblen = 0;
	int ioctl_response = 0;

    if (ctx == NULL) {
        goto out;
    }

	memset(&request, 0, sizeof(request));
	for (count=0; count<16; count++)
		request.guid[count] = count;
	request.timeout = 10000; // 10 sec timeout

	// initialize buffer with 0 values
	request.request_len = request_len;
	request.request_buffer = malloc(request.request_len);
	memset(request.request_buffer, 0x0, request.request_len);

	request.response_len = response_len;
	request.response_buffer = malloc(request.response_len);
	memset(request.response_buffer, 0x0, request.response_len);

	request.data_len = data_len;
        request.data_valid = 1;
	request.data_buffer = malloc(request.data_len);
	memset(request.data_buffer, 0x0, request.data_len);

	//sample blob format
	//"%NODE%/%ACCOUNT%/%CONTAINER%/%BLOBNAME%?sr=b&sk=system-1&sv=2014-02-14&sp=rw&se=9999-01-01,%DSASKEY%,0,%DOMAIN%"
	//"XDISK:0.0.0.0/md-pbkwsw54zpvl/xppxscqhzxzg/abcd?sr=b&sp=rw&se=9999-01-01&sk=system-1&sv=2014-02-14,$rzSYNQOjz4Rjb5tFVk4O0JQuK2CeKCW/R4kfiGDWfjc=$,0,z44.blob.storage.azure.net"

	bloblen = strlen(fda_bloburl);
	if(bloblen != fda_bloburl_length)
	{
		printf("blobstr length mismatch.");
		return FDA_INVALID_STATUS;
	}

	printf("Bloburl length = %d\n", bloblen);
	printf("Bloburl with DSAS key = %s\n", fda_bloburl);

	// Init Request Buffer
	// op_id = 1 for attach, op_id = 2 for detach operation
	req_buffer.op_id = FDA_DETACH; // 2 for detach op
	req_buffer.lun_num = fda_lun_number;
	memcpy(req_buffer.blobstr, fda_bloburl, strlen(fda_bloburl));
	memcpy(request.request_buffer, &req_buffer, bloblen+8);

    struct timespec ts1, ts2; // time latency calculation
	clock_gettime(CLOCK_MONOTONIC, &ts1);

	printf("Detach Op: Sending IOCTL to VSC driver\n");
	ioctl_response = ioctl(ctx->fvc_fd, IOCTL_XS_FASTPATH_DRIVER_USER_REQUEST, &request);
    printf("IOCTL response = %d errno %d\n", ioctl_response, errno);

	clock_gettime(CLOCK_MONOTONIC, &ts2);
    double qadLatencyMs = (1000.0*ts2.tv_sec + 1e-6*ts2.tv_nsec)
                          - (1000.0*ts1.tv_sec + 1e-6*ts1.tv_nsec);

	if (ioctl_response == 0)
	{
	    printf("IOCTL passed.\n");
		printf("QAD latency for IOCTL_XS_FASTPATH_DRIVER_USER_REQUEST response: %.3f ms.\n", qadLatencyMs);
		status = FDA_OP_SUCCEEDED;
	}
	else
	{
		perror("IOCTL failed.");
		printf("IOCTL failed: %Return value status %0X response_len %u\n", request.response.status, request.response.response_len);
		status = FDA_OP_FAILED;
	}

out:
    return status;
}

void fda_vsc_cleanup(fda_vsc_context_t *ctx)
{
    if (ctx == NULL) {
        return;
    }

    printf("\nClosing Driver\n");
    if (ctx->fvc_fd > 0) {
        close(ctx->fvc_fd);
    }

	free(ctx);
}
