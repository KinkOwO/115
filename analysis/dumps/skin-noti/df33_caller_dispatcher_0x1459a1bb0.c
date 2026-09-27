// caller_dispatcher_0x1459a1bb0

// sub_1459A1BB0
__int64 __fastcall sub_1459A1BB0(__int64 a1, __int64 a2, unsigned __int8 *a3)
{
  __int64 v7; // rcx
  __int64 v8; // r8
  __int64 v9; // r9
  int v10; // r13d
  __int64 v11; // rdx
  int v12; // r14d
  __int64 v13; // rbx
  int v14; // edi
  int v15; // eax
  unsigned int v16; // ebx
  __int64 v17; // rax
  __int64 v18; // rsi
  __int64 v19; // r8
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rcx
  __int64 v23; // rax
  unsigned __int64 v24; // rdx
  __int64 v25; // rcx
  __int64 v26; // rdi
  _QWORD *v27; // rbx
  unsigned __int8 v28; // bl
  __int64 v29; // rax
  void *v30; // rcx
  int v31; // eax
  _QWORD *v32; // rdx
  __int64 v33; // rax
  __int64 v34; // rax
  __int64 v35; // rdx
  __int64 v36; // r8
  __int64 v37; // r9
  __int16 v38; // ax
  int v39; // esi
  _QWORD *v40; // rdi
  int v41; // ebx
  int v42; // eax
  unsigned __int64 v43; // rdx
  __int64 v44; // rcx
  unsigned __int64 v45; // rdx
  __int64 v46; // rcx
  __int64 v47; // rbx
  __int64 v48; // rax
  __int64 v49; // rax
  __int64 v50; // r13
  __int64 v51; // rdx
  __int64 v52; // rcx
  __int64 v53; // r8
  __int64 v54; // r9
  int v55; // edi
  int v56; // ebx
  int v57; // eax
  __int64 v58; // rax
  __int64 v59; // r8
  int v60; // [rsp+20h] [rbp-69h]
  int v61; // [rsp+20h] [rbp-69h]
  _BYTE v62[4]; // [rsp+40h] [rbp-49h] BYREF
  int v63; // [rsp+44h] [rbp-45h] BYREF
  _BYTE v64[16]; // [rsp+48h] [rbp-41h] BYREF
  __int64 v65; // [rsp+58h] [rbp-31h]
  _QWORD v66[2]; // [rsp+60h] [rbp-29h] BYREF
  __int64 v67; // [rsp+70h] [rbp-19h]
  unsigned __int64 v68; // [rsp+78h] [rbp-11h]
  _QWORD v69[2]; // [rsp+80h] [rbp-9h] BYREF
  __int64 v70; // [rsp+90h] [rbp+7h]
  unsigned __int64 v71; // [rsp+98h] [rbp+Fh]

  v65 = -2;
  if ( !*(_BYTE *)(a1 + 8) )
    return 2147483649LL;
  sub_146EA2120(a3, *(unsigned int *)(a3 + 3));
  v10 = MEMORY[0xDC5CDC8]();
  v63 = v10;
  v11 = *a3;
  if ( *a3 )
  {
    if ( (_DWORD)v11 == 1 )
    {
      v62[0] = 0;
      sub_146EA09F0(v62, 1);
      v17 = sub_146E8C7D0(&unk_14A9000F8);
      v66[0] = 0;
      v67 = 0;
      v68 = 7;
      v18 = -1;
      v19 = -1;
      do
        ++v19;
      while ( *(_WORD *)(v17 + 2 * v19) );
      sub_14014C8D0(v66, v17);
      v21 = sub_146D74000(v20);
      sub_146D76380(v21, *(unsigned __int16 *)(a3 + 1));
      LOWORD(v63) = 0;
      if ( v62[0] )
      {
        v28 = 1;
      }
      else
      {
        sub_146EA1920(&v63);
        v23 = sub_146E8C7D0(&unk_14A900138);
        v24 = -1;
        do
          ++v24;
        while ( *(_WORD *)(v23 + 2 * v24) );
        v25 = v67;
        if ( v24 > v68 - v67 )
        {
          sub_1401E8C00((unsigned int)v66, v24, v62[0], v23, v24);
          v28 = 0;
        }
        else
        {
          v26 = v67 + v24;
          v67 += v24;
          v27 = v66;
          if ( v68 >= 8 )
            v27 = (_QWORD *)v66[0];
          sub_148AA1E60((char *)v27 + 2 * v25, v23, 2 * v24);
          *((_WORD *)v27 + v26) = 0;
          v28 = 0;
        }
      }
      v29 = sub_146D74000(v22);
      sub_146D776A0(v29, *(unsigned __int16 *)(a3 + 1), v28);
      v30 = &unk_1496F2238;
      if ( !v62[0] )
        v30 = &unk_14A900160;
      v31 = sub_146E8C7D0(v30);
      v32 = v66;
      if ( v68 >= 8 )
        v32 = (_QWORD *)v66[0];
      v61 = v31;
      v33 = sub_146E8CF20(v64, v32, qword_14EF38F60[*(unsigned __int16 *)(a3 + 1)]);
      v34 = sub_14014F430(v33);
      v69[0] = 0;
      v70 = 0;
      v71 = 7;
      do
        ++v18;
      while ( *(_WORD *)(v34 + 2 * v18) );
      sub_14014C8D0(v69, v34);
      sub_146E8C910(v64);
      v38 = *(_WORD *)(a3 + 1);
      if ( v38 != 714 && v38 != 1638 && v38 != 2111 )
      {
        v39 = sub_14021A860(2111, v35, v36, v37, v61);
        v40 = v69;
        if ( v71 >= 8 )
          v40 = (_QWORD *)v69[0];
        v41 = sub_146E8C7D0(&unk_14A9000C0);
        v42 = sub_146E8C7D0(&unk_14A8FFFE0);
        sub_146E939E0(v39, 0, v42, v41, 400, (__int64)&qword_14E683958, (__int64)v40);
      }
      sub_146FE3FB0();
      sub_146E0F670(v64, qword_14EF38F60[*(unsigned __int16 *)(a3 + 1)], *(unsigned __int16 *)(a3 + 1));
      v16 = sub_14599D200(a1, a3, v62[0], (unsigned __int16)v63);
      sub_146E0FC70(v64);
      sub_146FE3F50();
      if ( v71 >= 8 )
      {
        v43 = 2 * v71 + 2;
        v44 = v69[0];
        if ( v43 >= 0x1000 )
        {
          v43 = 2 * v71 + 41;
          v44 = *(_QWORD *)(v69[0] - 8LL);
          if ( (unsigned __int64)(v69[0] - v44 - 8) > 0x1F )
            sub_148AAF304(v44, v43);
        }
        sub_146E9F3A0(v44, v43);
      }
      v70 = 0;
      v71 = 7;
      LOWORD(v69[0]) = 0;
      if ( v68 >= 8 )
      {
        v45 = 2 * v68 + 2;
        v46 = v66[0];
        if ( v45 >= 0x1000 )
        {
          v45 = 2 * v68 + 41;
          v46 = *(_QWORD *)(v66[0] - 8LL);
          if ( (unsigned __int64)(v66[0] - v46 - 8) > 0x1F )
            sub_148AAF304(v46, v45);
        }
        sub_146E9F3A0(v46, v45);
      }
      v67 = 0;
      v68 = 7;
      LOWORD(v66[0]) = 0;
    }
    else
    {
      v12 = sub_14021A860(v7, v11, v8, v9, v60);
      v13 = sub_146E8C7D0(&unk_14A900180);
      v14 = sub_146E8C7D0(&unk_14A9000C0);
      v15 = sub_146E8C7D0(&unk_14A8FFFE0);
      sub_146E938E0(v12, 0, v15, v14, 434, (__int64)&qword_14E683940, v13);
      v16 = 0x80000000;
    }
  }
  else
  {
    v47 = qword_14EF334B0[*(unsigned __int16 *)(a3 + 1)];
    v48 = sub_146E8C7D0(&unk_14A900098);
    v49 = sub_146E8CF20(v64, v48, v47);
    v50 = sub_14014F430(v49);
    sub_146E8C910(v64);
    if ( *(_WORD *)(a3 + 1) != 570 )
    {
      v55 = sub_14021A860(v52, v51, v53, v54, v60);
      v56 = sub_146E8C7D0(&unk_14A9000C0);
      v57 = sub_146E8C7D0(&unk_14A8FFFE0);
      sub_146E939E0(v55, 0, v57, v56, 293, (__int64)&qword_14E683958, v50);
    }
    sub_146FE3FB0();
    sub_146E0F670(v64, qword_14EF334B0[*(unsigned __int16 *)(a3 + 1)], *(unsigned __int16 *)(a3 + 1));
    v16 = sub_14599D380(a1, a2, a3);
    sub_146E0FC70(v64);
    sub_146FE3F50();
    v10 = v63;
  }
  if ( (unsigned __int8)sub_1459AC4C0(qword_14E66C090) == 1 )
  {
    if ( sub_1413085D0(qword_14E66C090) )
    {
      v58 = sub_1413085D0(qword_14E66C090);
      if ( (unsigned int)sub_1410A7440(v58) == 4 )
      {
        v59 = (unsigned int)MEMORY[0xDC5CDC8]() - v10;
        if ( *a3 )
        {
          if ( *a3 == 1 )
            sub_146659DD0(qword_14E683C68, *(unsigned __int16 *)(a3 + 1), v59);
        }
        else
        {
          sub_146659E10(qword_14E683C68, *(unsigned __int16 *)(a3 + 1), v59);
        }
      }
    }
  }
  return v16;
}

