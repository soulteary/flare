#!/usr/bin/env python3
"""Generate diagnostic-only copied source instrumentation; never edits production tree.

Counts Go commit/atomicWrite/Sync invocations in internal/security/store.go only,
not kernel syscalls or unrelated YAML persistence. Native timings use no overlay.
"""
import argparse
import hashlib
import json
import pathlib

HELPER = '''package security
import ("os"; "sync/atomic")
var transactionCommit, transactionAtomic, transactionFileSync, transactionDirSync, transactionSyncErrors atomic.Uint64
var transactionJournal, transactionState, transactionComplete atomic.Uint64
func transactionCountAtomic(name string) {
 transactionAtomic.Add(1)
 switch name {case "commit.json":transactionJournal.Add(1);case "state.json":transactionState.Add(1);case "complete":transactionComplete.Add(1)}
}
func transactionCountSync(f *os.File) error {
 info,err:=f.Stat(); if err==nil && info.IsDir(){transactionDirSync.Add(1)}else{transactionFileSync.Add(1)}
 err=f.Sync();if err!=nil{transactionSyncErrors.Add(1)};return err
}
func TransactionDiagnosticCounters() map[string]uint64 {return map[string]uint64{
 "commit_calls":transactionCommit.Load(),"atomic_write_calls":transactionAtomic.Load(),
 "journal_write_calls":transactionJournal.Load(),"state_write_calls":transactionState.Load(),"complete_write_calls":transactionComplete.Load(),
 "file_sync_calls":transactionFileSync.Load(),"directory_sync_calls":transactionDirSync.Load(),"sync_errors":transactionSyncErrors.Load(),
}}
'''


def instrument(root):
    root = pathlib.Path(root).resolve()
    repository = pathlib.Path(__file__).resolve().parents[2]
    if root == repository or (root / '.git').exists():
        raise ValueError('diagnostic overlay requires an isolated source copy without .git')
    store = root / 'internal/security/store.go'
    original = store.read_text()
    text = original
    for signature, insertion in [
        ('func (s *Store) commit(next state) error {', '\n transactionCommit.Add(1)'),
        ('func (s *Store) atomicWrite(name string, b []byte) error {', '\n transactionCountAtomic(name)'),
    ]:
        if text.count(signature) != 1:
            raise ValueError('expected unique instrumentation boundary: ' + signature)
        text = text.replace(signature, signature + insertion)
    sync_count = text.count('f.Sync()') + text.count('d.Sync()')
    if sync_count < 3:
        raise ValueError('security Sync callsites missing')
    text = text.replace('f.Sync()', 'transactionCountSync(f)').replace('d.Sync()', 'transactionCountSync(d)')
    wrapper = root / 'main_profile_test.go'
    wrapper_original = wrapper.read_text()
    wrapper_text = wrapper_original.replace('"encoding/json"', '"encoding/json"\n "github.com/soulteary/flare/internal/security"')
    marker = 'mux := http.NewServeMux()'
    if wrapper_text.count(marker) != 1:
        raise ValueError('expected unique localhost test mux')
    wrapper_text = wrapper_text.replace(marker, marker + '\n mux.HandleFunc("/counters",func(w http.ResponseWriter,r *http.Request){_ = json.NewEncoder(w).Encode(security.TransactionDiagnosticCounters())})')
    store.write_text(text)
    (root / 'internal/security/transaction_diagnostic.go').write_text(HELPER)
    wrapper.write_text(wrapper_text)
    manifest = {
        'scope': 'diagnostic copied-source only; Go invocations, not OS syscalls',
        'sync_callsites_wrapped': sync_count,
        'store_original_sha256': hashlib.sha256(original.encode()).hexdigest(),
        'store_instrumented_sha256': hashlib.sha256(text.encode()).hexdigest(),
        'wrapper_original_sha256': hashlib.sha256(wrapper_original.encode()).hexdigest(),
        'helper_sha256': hashlib.sha256(HELPER.encode()).hexdigest(),
    }
    (root / 'transaction-overlay-manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
    return manifest

if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('copied_source_directory')
    print(json.dumps(instrument(parser.parse_args().copied_source_directory),indent=2))
