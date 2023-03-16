#ifndef FDA_INTERFACE_H
#define FDA_INTERFACE_H

#include "fda_rq_context.h"

/*
 * Interfaces to be consumed by DiskRP (?)
 *
 * For the PoC,these interfaces are synchronous. This will change in production.
 *
 * The interfaces are essentially wrappers around ioctl(2) invocations on
 * /dev/azure_blob, to send fast disk attach/detach requests over to xdisksvc on
 * the Host VM.
 */

fda_vsc_context_t *fda_vsc_init();

int fda_disk_attach(fda_vsc_context_t *ctx,
    const char *fda_bloburl,
    uint32_t fda_bloburl_length,
    uint32_t fda_lun_number
);

int fda_disk_detach(fda_vsc_context_t *ctx,
    const char *fda_bloburl,
    uint32_t fda_bloburl_length,
    uint32_t fda_lun_number
);

void fda_vsc_cleanup(fda_vsc_context_t *ctx);


#endif /* FDA_INTERFACE_H */
