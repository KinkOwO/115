// callee_sub_14449CAF0

__int64 __fastcall sub_14449CAF0(__int64 a1)
{
  _QWORD *v2; // rax
  __int64 v3; // rdx
  __int64 v4; // r8
  _QWORD *v5; // rax
  __int64 v6; // rax
  __int64 v7; // rax

  *(_QWORD *)a1 = off_14A300CA8;
  sub_146EBB4E0(a1 + 8);
  v2 = (_QWORD *)(a1 + 64);
  v3 = 8;
  v4 = 8;
  do
  {
    *v2++ = 0;
    --v4;
  }
  while ( v4 );
  v5 = (_QWORD *)(a1 + 128);
  do
  {
    *v5 = -1;
    v5[1] = 0;
    v5[2] = 0;
    v5 += 3;
    --v3;
  }
  while ( v3 );
  *(_BYTE *)(a1 + 352) = 0;
  *(_QWORD *)(a1 + 356) = 0;
  *(_QWORD *)(a1 + 364) = 0;
  *(_QWORD *)(a1 + 372) = 0;
  *(_QWORD *)(a1 + 380) = 0;
  *(_QWORD *)(a1 + 388) = 0;
  *(_QWORD *)(a1 + 400) = 0;
  *(_QWORD *)(a1 + 408) = 0;
  v6 = sub_146E8BA20(40);
  *(_QWORD *)v6 = v6;
  *(_QWORD *)(v6 + 8) = v6;
  *(_QWORD *)(v6 + 16) = v6;
  *(_WORD *)(v6 + 24) = 257;
  *(_QWORD *)(a1 + 400) = v6;
  sub_146E9F7D0(a1 + 424);
  sub_146E9F7D0(a1 + 456);
  *(_QWORD *)(a1 + 320) = -1;
  *(_QWORD *)(a1 + 328) = -1;
  *(_QWORD *)(a1 + 336) = -1;
  *(_QWORD *)(a1 + 344) = -1;
  *(_WORD *)(a1 + 488) = 0;
  v7 = sub_146E8C7D0(&unk_14A300CC0);
  sub_14449E360(a1, v7);
  sub_14599D5D0(qword_14E66C090, 1552, sub_14449D400, 0);
  sub_14599D5D0(qword_14E66C090, 1553, sub_14449D2E0, 0);
  sub_14599D5D0(qword_14E66C090, 1551, sub_14449CDB0, 0);
  sub_14599D5D0(qword_14E66C090, 1554, sub_14449D200, 0);
  sub_14599D5D0(qword_14E66C090, 1555, nullsub_1, 0);
  sub_14599D5D0(qword_14E66C090, 1556, sub_14449CEA0, 0);
  sub_14599D5D0(qword_14E66C090, 1557, sub_14449D040, 0);
  return a1;
}

