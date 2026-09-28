// reader_sub_141FB2F60

void __fastcall sub_141FB2F60(__int64 a1)
{
  __int64 v2; // r13
  __int64 v3; // rax
  __int64 v4; // rax
  volatile signed __int32 *v5; // rbx
  __int64 *v6; // r14
  __int64 v7; // rdi
  __int64 v8; // r12
  __int64 v9; // rbx
  float v10; // xmm6_4
  double v11; // xmm0_8
  float v12; // xmm6_4
  double v13; // xmm0_8
  volatile signed __int32 *v14; // rbx
  float v15; // xmm8_4
  float v16; // xmm7_4
  __int64 v17; // rdi
  void (__fastcall *v18)(__int64); // rbx
  __int64 v19; // rax
  __int64 v20; // rax
  volatile signed __int32 *v21; // rbx
  __int64 v22; // rbx
  double v23; // xmm0_8
  __int64 v24; // rcx
  __int128 *v25; // rax
  __int128 *v26; // rdi
  unsigned __int64 v27; // r14
  __int64 v28; // rbx
  unsigned int v29; // eax
  __int64 v30; // rdi
  void (__fastcall *v31)(__int64, __int64); // rbx
  __int128 *v32; // rdx
  __int64 v33; // rax
  __int64 v34; // rax
  unsigned __int64 v35; // rdx
  __int64 v36; // rcx
  volatile signed __int32 *v37; // rbx
  volatile signed __int32 *v38; // rbx
  volatile signed __int32 *v39; // rbx
  float v40; // [rsp+20h] [rbp-E0h] BYREF
  float v41; // [rsp+24h] [rbp-DCh]
  __int64 v42; // [rsp+28h] [rbp-D8h] BYREF
  volatile signed __int32 *v43; // [rsp+30h] [rbp-D0h]
  _DWORD v44[2]; // [rsp+38h] [rbp-C8h] BYREF
  _DWORD v45[4]; // [rsp+40h] [rbp-C0h] BYREF
  __int64 v46; // [rsp+50h] [rbp-B0h] BYREF
  volatile signed __int32 *v47; // [rsp+58h] [rbp-A8h]
  __int64 v48; // [rsp+60h] [rbp-A0h] BYREF
  volatile signed __int32 *v49; // [rsp+68h] [rbp-98h]
  __int64 v50; // [rsp+70h] [rbp-90h] BYREF
  volatile signed __int32 *v51; // [rsp+78h] [rbp-88h]
  __int64 v52; // [rsp+80h] [rbp-80h]
  _BYTE v53[8]; // [rsp+88h] [rbp-78h] BYREF
  volatile signed __int32 *v54; // [rsp+90h] [rbp-70h]
  _BYTE v55[8]; // [rsp+98h] [rbp-68h] BYREF
  volatile signed __int32 *v56; // [rsp+A0h] [rbp-60h]
  _BYTE v57[16]; // [rsp+A8h] [rbp-58h] BYREF
  __int128 v58; // [rsp+B8h] [rbp-48h] BYREF
  unsigned __int64 v59; // [rsp+C8h] [rbp-38h]
  unsigned __int64 v60; // [rsp+D0h] [rbp-30h]

  v52 = -2;
  *(_BYTE *)(a1 + 2160) = 0;
  v2 = sub_141FAB150();
  if ( v2 )
  {
    (*(void (__fastcall **)(__int64, __int64 *))(*(_QWORD *)a1 + 272LL))(a1, &v50);
    if ( v50 )
    {
      sub_141FAB560(v2);
      sub_146EC9EB0(*(_QWORD *)(a1 + 208), &v40);
      v41 = 2.0;
      v3 = sub_146E8C7D0(&unk_1498CD328);
      v4 = sub_145F6E4E0(a1, v53, v3);
      sub_14088E780(&v46, v4);
      v5 = v54;
      if ( v54 )
      {
        if ( _InterlockedExchangeAdd(v54 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v5)(v5);
          if ( _InterlockedExchangeAdd(v5 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v5 + 8LL))(v5);
        }
      }
      v6 = (__int64 *)(a1 + 1592);
      v7 = a1 + 1512;
      v8 = 5;
      do
      {
        sub_14111F0C0(v7, &v42);
        if ( v42 )
        {
          sub_141FB2C80(v7);
          if ( *v6 )
          {
            if ( (unsigned __int8)sub_141FB6530(*v6) )
            {
              v9 = v42;
              v10 = v41;
              v11 = sub_146EC9590(v42);
              v44[0] = LODWORD(v11);
              *(float *)&v44[1] = v10;
              sub_146ECC720(v9, v44);
              v12 = sub_146EC9600(v42);
              v13 = sub_146EC9470(v42);
              v41 = (float)(v12 + *(float *)&v13) + 5.0;
            }
          }
        }
        v14 = v43;
        if ( v43 )
        {
          if ( _InterlockedExchangeAdd(v43 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v14)(v14);
            if ( _InterlockedExchangeAdd(v14 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v14 + 8LL))(v14);
          }
        }
        v7 += 120;
        v6 += 15;
        --v8;
      }
      while ( v8 );
      v15 = v40;
      v16 = v41;
      v17 = v46;
      if ( v46 )
      {
        v18 = *(void (__fastcall **)(__int64))(*(_QWORD *)v46 + 320LL);
        sub_146ECA0B0(v46);
        v18(v17);
        v16 = v16 + sub_146EC9600(v46);
      }
      v19 = sub_146E8C7D0(&unk_1498CD348);
      v20 = sub_145F6E4E0(a1, v55, v19);
      sub_1401E9C70(&v48, v20);
      v21 = v56;
      if ( v56 )
      {
        if ( _InterlockedExchangeAdd(v56 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
          if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
        }
      }
      v22 = v48;
      if ( v48 )
      {
        v23 = sub_146EC9590(v48);
        v45[0] = LODWORD(v23);
        *(float *)&v45[1] = v16 + 4.0;
        sub_146ECC720(v22, v45);
      }
      v24 = *(_QWORD *)(a1 + 2112);
      if ( v24 )
      {
        v25 = (__int128 *)sub_1447D3DF0(v24);
        v26 = v25;
        *(_QWORD *)&v58 = 0;
        v59 = 0;
        v60 = 0;
        v27 = *((_QWORD *)v25 + 2);
        if ( *((_QWORD *)v25 + 3) >= 8u )
          v26 = *(__int128 **)v25;
        if ( v27 >= 8 )
        {
          v28 = v27 | 7;
          if ( (v27 | 7) > 0x7FFFFFFFFFFFFFFELL )
            v28 = 0x7FFFFFFFFFFFFFFELL;
          *(_QWORD *)&v58 = sub_14014CB50(&v58, v28 + 1);
          sub_148AA1E60(v58, v26, 2 * v27 + 2);
          v60 = v28;
        }
        else
        {
          v58 = *v26;
          v60 = 7;
        }
        v59 = v27;
        v29 = sub_141FAB840(v2);
        v30 = *(_QWORD *)(a1 + 2112);
        v31 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v30 + 688LL);
        v32 = &v58;
        if ( v60 >= 8 )
          v32 = (__int128 *)v58;
        v33 = sub_146E8CF20(v57, v32, v29);
        v34 = sub_14014F430(v33);
        v31(v30, v34);
        sub_146E8C910(v57);
        if ( v60 >= 8 )
        {
          v35 = 2 * v60 + 2;
          v36 = v58;
          if ( v35 >= 0x1000 )
          {
            v35 = 2 * v60 + 41;
            v36 = *(_QWORD *)(v58 - 8);
            if ( (unsigned __int64)(v58 - v36 - 8) > 0x1F )
              sub_148AAF304(v36, v35);
          }
          sub_146E9F3A0(v36, v35);
        }
        v59 = 0;
        v60 = 7;
        LOWORD(v58) = 0;
      }
      sub_1467AB860(a1, (unsigned int)(int)v15, (unsigned int)((int)v16 + 40));
      v37 = v49;
      if ( v49 )
      {
        if ( _InterlockedExchangeAdd(v49 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v37)(v37);
          if ( _InterlockedExchangeAdd(v37 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v37 + 8LL))(v37);
        }
      }
      v38 = v47;
      if ( v47 )
      {
        if ( _InterlockedExchangeAdd(v47 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v38)(v38);
          if ( _InterlockedExchangeAdd(v38 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v38 + 8LL))(v38);
        }
      }
    }
    v39 = v51;
    if ( v51 && _InterlockedExchangeAdd(v51 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v39)(v39);
      if ( _InterlockedExchangeAdd(v39 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v39 + 8LL))(v39);
    }
  }
}

