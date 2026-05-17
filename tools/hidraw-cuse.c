#define FUSE_USE_VERSION 31

#include <cuse_lowlevel.h>
#include <errno.h>
#include <fcntl.h>
#include <fuse_lowlevel.h>
#include <linux/hidraw.h>
#include <linux/input.h>
#include <pthread.h>
#include <poll.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

static const char *device_name = "RpiKeyboardSwitcher E2E Keyboard";

static const unsigned char report_descriptor[] = {
	0x05, 0x01, 0x09, 0x06, 0xa1, 0x01, 0x85, 0x01,
	0x05, 0x07, 0x19, 0xe0, 0x29, 0xe7, 0x15, 0x00,
	0x25, 0x01, 0x75, 0x01, 0x95, 0x08, 0x81, 0x02,
	0x95, 0x01, 0x75, 0x08, 0x81, 0x01, 0x95, 0x05,
	0x75, 0x01, 0x05, 0x08, 0x19, 0x01, 0x29, 0x05,
	0x91, 0x02, 0x95, 0x01, 0x75, 0x03, 0x91, 0x01,
	0x95, 0x06, 0x75, 0x08, 0x15, 0x00, 0x25, 0x65,
	0x05, 0x07, 0x19, 0x00, 0x29, 0x65, 0x81, 0x00,
	0xc0,
};

static const unsigned char input_reports[][9] = {
	{0x01, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00},
	{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
};

struct hidraw_state {
	const char *devname;
	const char *path_file;
	const char *trigger_file;
	pthread_mutex_t lock;
	size_t next_report;
	bool triggered;
	struct fuse_pollhandle *pollhandle;
};

static void reply_error(fuse_req_t req, int err)
{
	fuse_reply_err(req, err < 0 ? -err : err);
}

static void hidraw_open(fuse_req_t req, struct fuse_file_info *fi)
{
	fuse_reply_open(req, fi);
}

static void hidraw_read(fuse_req_t req, size_t size, off_t off,
			struct fuse_file_info *fi)
{
	struct hidraw_state *state = fuse_req_userdata(req);
	const unsigned char *report = NULL;
	size_t report_size = 0;

	(void)off;
	(void)fi;

	pthread_mutex_lock(&state->lock);
	if (state->triggered &&
	    state->next_report < sizeof(input_reports) / sizeof(input_reports[0])) {
		report = input_reports[state->next_report];
		report_size = sizeof(input_reports[state->next_report]);
		fprintf(stderr, "read report %zu\n", state->next_report);
		state->next_report++;
	}
	pthread_mutex_unlock(&state->lock);

	if (!report) {
		reply_error(req, EAGAIN);
		return;
	}
	if (size < report_size)
		report_size = size;

	fuse_reply_buf(req, (const char *)report, report_size);
}

static void hidraw_poll(fuse_req_t req, struct fuse_file_info *fi,
			struct fuse_pollhandle *ph)
{
	struct hidraw_state *state = fuse_req_userdata(req);
	unsigned revents = 0;
	struct fuse_pollhandle *old = NULL;

	(void)fi;

	pthread_mutex_lock(&state->lock);
	if (state->triggered &&
	    state->next_report < sizeof(input_reports) / sizeof(input_reports[0])) {
		revents = POLLIN;
		fprintf(stderr, "poll ready\n");
	} else if (ph) {
		old = state->pollhandle;
		state->pollhandle = ph;
		ph = NULL;
	}
	pthread_mutex_unlock(&state->lock);

	if (old)
		fuse_pollhandle_destroy(old);
	if (ph)
		fuse_pollhandle_destroy(ph);
	fuse_reply_poll(req, revents);
}

static bool retry_output_ioctl(fuse_req_t req, void *arg, size_t size,
			       size_t out_bufsz)
{
	struct iovec out_iov;

	if (out_bufsz != 0)
		return false;

	out_iov.iov_base = arg;
	out_iov.iov_len = size;
	fuse_reply_ioctl_retry(req, NULL, 0, &out_iov, 1);

	return true;
}

static void hidraw_ioctl(fuse_req_t req, int cmd, void *arg,
			 struct fuse_file_info *fi, unsigned int flags,
			 const void *in_buf, size_t in_bufsz,
			 size_t out_bufsz)
{
	(void)arg;
	(void)fi;
	(void)flags;
	(void)in_buf;
	(void)in_bufsz;
	(void)out_bufsz;

	if (_IOC_TYPE(cmd) != 'H') {
		reply_error(req, ENOTTY);
		return;
	}

	switch (_IOC_NR(cmd)) {
	case 0x01: {
		int size = sizeof(report_descriptor);
		if (retry_output_ioctl(req, arg, sizeof(size), out_bufsz))
			return;
		fuse_reply_ioctl(req, 0, &size, sizeof(size));
		return;
	}
	case 0x02: {
		struct hidraw_report_descriptor descriptor;
		if (retry_output_ioctl(req, arg, sizeof(descriptor), out_bufsz))
			return;
		memset(&descriptor, 0, sizeof(descriptor));
		descriptor.size = sizeof(report_descriptor);
		memcpy(descriptor.value, report_descriptor, sizeof(report_descriptor));
		fuse_reply_ioctl(req, 0, &descriptor, sizeof(descriptor));
		return;
	}
	case 0x03: {
		struct hidraw_devinfo info;
		if (retry_output_ioctl(req, arg, sizeof(info), out_bufsz))
			return;
		memset(&info, 0, sizeof(info));
		info.bustype = BUS_USB;
		info.vendor = 0x1209;
		info.product = 0x0001;
		fuse_reply_ioctl(req, 0, &info, sizeof(info));
		return;
	}
	case 0x04: {
		size_t size = _IOC_SIZE(cmd);
		char name[256];
		if (size == 0 || size > sizeof(name))
			size = sizeof(name);
		if (retry_output_ioctl(req, arg, size, out_bufsz))
			return;
		memset(name, 0, sizeof(name));
		snprintf(name, sizeof(name), "%s", device_name);
		fuse_reply_ioctl(req, 0, name, size);
		return;
	}
	default:
		reply_error(req, ENOTTY);
		return;
	}
}

static void hidraw_init_done(void *userdata)
{
	struct hidraw_state *state = userdata;
	FILE *file;

	if (!state->path_file)
		return;

	file = fopen(state->path_file, "w");
	if (!file)
		return;
	fprintf(file, "/dev/%s\n", state->devname);
	fclose(file);
}

static void hidraw_destroy(void *userdata)
{
	struct hidraw_state *state = userdata;
	struct fuse_pollhandle *ph = NULL;

	pthread_mutex_lock(&state->lock);
	ph = state->pollhandle;
	state->pollhandle = NULL;
	pthread_mutex_unlock(&state->lock);

	if (ph)
		fuse_pollhandle_destroy(ph);
}

static void *trigger_thread(void *userdata)
{
	struct hidraw_state *state = userdata;

	while (access(state->trigger_file, F_OK) != 0)
		usleep(50000);

	pthread_mutex_lock(&state->lock);
	state->triggered = true;
	state->next_report = 0;
	struct fuse_pollhandle *ph = state->pollhandle;
	state->pollhandle = NULL;
	pthread_mutex_unlock(&state->lock);

	if (ph) {
		fuse_lowlevel_notify_poll(ph);
		fuse_pollhandle_destroy(ph);
	}
	fprintf(stderr, "input reports queued\n");

	return NULL;
}

static const struct cuse_lowlevel_ops hidraw_ops = {
	.open = hidraw_open,
	.read = hidraw_read,
	.poll = hidraw_poll,
	.ioctl = hidraw_ioctl,
	.init_done = hidraw_init_done,
	.destroy = hidraw_destroy,
};

static const char *arg_value(int argc, char **argv, const char *name,
			     const char *fallback)
{
	for (int i = 1; i + 1 < argc; i++) {
		if (strcmp(argv[i], name) == 0)
			return argv[i + 1];
	}
	return fallback;
}

int main(int argc, char **argv)
{
	struct hidraw_state state = {
		.devname = arg_value(argc, argv, "--name", "rpi-hidraw-e2e"),
		.path_file = arg_value(argc, argv, "--path-file", "/tmp/hidraw.path"),
		.trigger_file = arg_value(argc, argv, "--trigger-file", "/tmp/send-report"),
		.lock = PTHREAD_MUTEX_INITIALIZER,
	};
	const char *dev_info_argv[1];
	char devname_arg[128];
	struct cuse_info cuse_info;
	char *fuse_argv[] = {argv[0], "-f", "-s"};
	pthread_t thread;

	snprintf(devname_arg, sizeof(devname_arg), "DEVNAME=%s", state.devname);
	dev_info_argv[0] = devname_arg;

	memset(&cuse_info, 0, sizeof(cuse_info));
	cuse_info.dev_info_argc = 1;
	cuse_info.dev_info_argv = dev_info_argv;
	cuse_info.flags = CUSE_UNRESTRICTED_IOCTL;

	if (pthread_create(&thread, NULL, trigger_thread, &state) != 0) {
		perror("pthread_create");
		return 1;
	}
	pthread_detach(thread);

	return cuse_lowlevel_main(3, fuse_argv, &cuse_info, &hidraw_ops, &state);
}
