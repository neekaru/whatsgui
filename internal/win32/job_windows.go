package win32

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcessJob owns one profile worker and all its WebView2 descendants.
// Closing the parent's handle also ends them if the parent crashes.
type ProcessJob struct{ handle windows.Handle }

func ContainProcess(pid int) (*ProcessJob, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	return &ProcessJob{handle: job}, nil
}

func (job *ProcessJob) Close() error {
	if job == nil {
		return nil
	}
	defer windows.CloseHandle(job.handle)
	// Let browser/cache processes flush profile data after the worker exits.
	// Force termination only when that graceful shutdown deadline expires.
	var accounting struct {
		TotalUserTime, TotalKernelTime, PeriodUserTime, PeriodKernelTime int64
		PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
	}
	deadline := time.Now().Add(5 * time.Second)
	terminated := false
	for {
		if err := windows.QueryInformationJobObject(job.handle, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
			return fmt.Errorf("memeriksa proses profil: %w", err)
		}
		if accounting.ActiveProcesses == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			if terminated {
				return errors.New("proses WebView2 belum selesai ditutup")
			}
			if err := windows.TerminateJobObject(job.handle, 0); err != nil {
				return err
			}
			terminated = true
			deadline = time.Now().Add(5 * time.Second)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
