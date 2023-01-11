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

#ifndef MOCK_IOCTL
    ctx->fvc_fd = open("/dev/azure_blob", O_RDWR);
    if (ctx->fvc_fd < 0) {
        perror("VSC init failed");
        goto err_out_free;
    }
#endif

out:
    return ctx;

#ifndef MOCK_IOCTL
err_out_free:
#endif
    free(ctx);
    ctx = NULL;
    return ctx;
}

static int fda_init_request(fda_request_t *req,
    const char *fda_activity_id,
    uint32_t fda_activity_id_length,
    const char *fda_vmid,
    uint32_t fda_vmid_length,
    const char *fda_bloburl,
    uint32_t fda_bloburl_length,
    const char *fda_dsas_key,
    uint32_t fda_dsas_key_length)
{
    int ret = -1;
    uint32_t data_offset = 0;
    char *copy_dest = NULL;
    void *mc_ret = NULL;

    copy_dest = (char *) req->fbi_data + data_offset;
    mc_ret = memcpy((void *) copy_dest, fda_activity_id, fda_activity_id_length);
    if (copy_dest != mc_ret) {
        perror("memcpy activity id failed");
        goto err_out;
    }
    data_offset += fda_activity_id_length;

    copy_dest = (char *) req->fbi_data + data_offset;
    mc_ret = memcpy((void *) copy_dest, fda_vmid, fda_vmid_length);
    if (copy_dest != mc_ret) {
        perror("memcpy vmid failed");
        goto err_out;
    }
    data_offset += fda_vmid_length;

    copy_dest = (char *) req->fbi_data + data_offset;
    mc_ret = memcpy((void *) copy_dest, fda_bloburl, fda_bloburl_length);
    if (copy_dest != mc_ret) {
        perror("memcpy bloburl failed");
        goto err_out;
    }
    data_offset += fda_bloburl_length;

    copy_dest = (char *) req->fbi_data + data_offset;
    mc_ret = memcpy((void *) copy_dest, fda_dsas_key, fda_dsas_key_length);
    if (copy_dest != mc_ret) {
        perror("memcpy dsas key failed");
        goto err_out;
    }
    data_offset += fda_dsas_key_length;

    ret = 0;

err_out:
    return ret;

}

int fda_disk_attach(fda_vsc_context_t *ctx,
    const char *fda_activity_id,
    uint32_t fda_activity_id_length,
    const char *fda_vmid,
    uint32_t fda_vmid_length,
    const char *fda_bloburl,
    uint32_t fda_bloburl_length,
    const char *fda_dsas_key,
    uint32_t fda_dsas_key_length)
{
    int status = FDA_OP_FAILED;
    int ret = 0;
    uint32_t data_length = 0;
    fda_request_t *req = NULL;

    if (ctx == NULL) {
        goto out;
    }

    data_length = fda_activity_id_length    +
                  fda_vmid_length           +
                  fda_bloburl_length        +
                  fda_dsas_key_length;
    req = malloc(sizeof(fda_request_t) + data_length);
    if (req == NULL) {
        perror("unable to allocate request");
        goto out;
    }

    req->fbi_magic = FDA_RQ_MAGIC;
    req->fbi_version = FDA_RQ_VERSION;
    req->fbi_rq_state = FDA_INVALID_REQUEST_STATE;
    req->fbi_op = FDA_ATTACH;
    req->fbi_op_status = FDA_OP_FAILED;

    ret = fda_init_request(req,
            fda_activity_id,
            fda_activity_id_length,
            fda_vmid,
            fda_vmid_length,
            fda_bloburl,
            fda_bloburl_length,
            fda_dsas_key,
            fda_dsas_key_length);
    if (ret < 0) {
        goto out;
    }

    /* XXX placeholder */
    //ret = ioctl(ctx->fvc_fd, _IOWR(), req);

    //XXX validate req->fbi_op_status;

#ifdef MOCK_IOCTL
    status = FDA_OP_SUCCEEDED;
#endif

out:
    if (req != NULL) {
        free(req);
    }
    return status;
}

int fda_disk_detach(fda_vsc_context_t *ctx,
    const char *fda_activity_id,
    uint32_t fda_activity_id_length,
    const char *fda_vmid,
    uint32_t fda_vmid_length,
    const char *fda_bloburl,
    uint32_t fda_bloburl_length,
    const char *fda_dsas_key,
    uint32_t fda_dsas_key_length)
{
    int status = FDA_OP_FAILED;
    int ret = 0;
    uint32_t data_length = 0;
    fda_request_t *req = NULL;

    if (ctx == NULL) {
        goto out;
    }

    data_length = fda_activity_id_length    +
                  fda_vmid_length           +
                  fda_bloburl_length        +
                  fda_dsas_key_length;
    req = malloc(sizeof(fda_request_t) + data_length);
    if (req == NULL) {
        perror("unable to allocate request");
        goto out;
    }

    req->fbi_magic = FDA_RQ_MAGIC;
    req->fbi_version = FDA_RQ_VERSION;
    req->fbi_rq_state = FDA_INVALID_REQUEST_STATE;
    req->fbi_op = FDA_DETACH;
    req->fbi_op_status = FDA_OP_FAILED;

    ret = fda_init_request(req,
            fda_activity_id,
            fda_activity_id_length,
            fda_vmid,
            fda_vmid_length,
            fda_bloburl,
            fda_bloburl_length,
            fda_dsas_key,
            fda_dsas_key_length);
    if (ret < 0) {
        goto out;
    }

    /* XXX placeholder */
    //ret = ioctl(ctx->fvc_fd, _IOWR(), req);

    //XXX validate req->fbi_op_status;

#ifdef MOCK_IOCTL
    status = FDA_OP_SUCCEEDED;
#endif

out:
    if (req != NULL) {
        free(req);
    }
    return status;

}


void fda_vsc_cleanup(fda_vsc_context_t *ctx)
{
    if (ctx == NULL) {
        return;
    }

    if (ctx->fvc_fd > 0) {
        close(ctx->fvc_fd);
    }
    free(ctx);
}
