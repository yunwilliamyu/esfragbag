/*
Package hhm parses hhm files generated by the HHsuite programs. (i.e., hhmake,
hhsearch, hhblits, etc.)

Each HHM file can be thought of as contain four logical sections: 1) The header
with meta information about the HMM. 2) Optional secondary structure
information from DSSP and/or PSIPED. 3) A multiple sequence alignment in A3M
format. 4) The HMM formatted similarly to HMMER's hmm files, but without pseudo
counts.
*/
package hhm
