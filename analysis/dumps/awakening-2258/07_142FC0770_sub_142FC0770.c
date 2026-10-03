// sub_142FC0770  size=826

void __fastcall sub_142FC0770(__int64 a1, __int64 a2)
{
  __int64 v3; // rax
  __int64 v4; // r8
  __int64 v5; // r15
  unsigned __int8 (__fastcall *v6)(__int64, __int64, __int64); // r9
  __int64 v7; // rcx
  __int64 v8; // rax
  __int64 v9; // rdx
  int v10; // r12d
  unsigned int v11; // r13d
  __int64 v12; // rax
  unsigned __int64 *i; // rdi
  unsigned __int64 *v14; // r14
  unsigned __int64 v15; // rbx
  __int64 *v16; // rdx
  __int64 *v17; // rax
  __int64 *v18; // rcx
  __int64 *v19; // rbx
  __int64 v20; // rdx
  __int64 v21; // rax
  __int64 v22; // rcx
  __int64 v23; // rax
  int v24; // ebx
  __int64 v25; // rax
  __int64 v26; // rcx
  unsigned __int64 v27; // rdx
  __int128 v28; // [rsp+20h] [rbp-E0h] BYREF
  __int64 v29; // [rsp+30h] [rbp-D0h]
  __int64 v30; // [rsp+38h] [rbp-C8h]
  _QWORD v31[2]; // [rsp+40h] [rbp-C0h] BYREF
  int v32; // [rsp+50h] [rbp-B0h]
  int v33; // [rsp+54h] [rbp-ACh]
  int v34; // [rsp+60h] [rbp-A0h]
  int v35; // [rsp+64h] [rbp-9Ch]
  int v36; // [rsp+68h] [rbp-98h]
  int v37; // [rsp+7Ch] [rbp-84h]
  __int64 v38; // [rsp+80h] [rbp-80h]
  int v39; // [rsp+98h] [rbp-68h]
  const char *v40; // [rsp+A0h] [rbp-60h]
  __int16 v41; // [rsp+C4h] [rbp-3Ch]
  int v42; // [rsp+C8h] [rbp-38h]

  if ( a2 != 0 )
  {
    v30 = -2;
    v3 = sub_1450BE260(a2);
    v5 = v3;
    if ( v3 != 0 )
    {
      v6 = *(unsigned __int8 (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)v3 + 3792LL);
      v7 = *(_QWORD *)(a1 + 160);
      if ( v7 != 0 && *(_DWORD *)(v7 + 8) != 0 )
        v8 = *(_QWORD *)(a1 + 168);
      else
        v8 = 0;
      v9 = v8 - 48;
      if ( v8 == 0 )
        v9 = 0;
      LOBYTE(v4) = 1;
      if ( v6(v5, v9, v4) != 0 )
      {
        v10 = (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)a1 + 1632LL))(a1, 25);
        v11 = (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)a1 + 1632LL))(a1, 24);
        v28 = 0;
        v29 = 0;
        __wind
        {
          v12 = sub_145383860(a1);
          sub_145F440A0(v12, &v28, 2258);
          v14 = *((unsigned __int64 **)&v28 + 1);
          for ( i = (unsigned __int64 *)v28; i != v14; ++i )
          {
            v15 = *i;
            if ( *i != 0 && (unsigned __int8)sub_145F38210(*i, 12) != 0 )
            {
              v16 = *(__int64 **)(a1 + 1120);
              v17 = (__int64 *)v16[1];
              v18 = v16;
              while ( *((_BYTE *)v17 + 25) == 0 )
              {
                if ( v17[4] >= v15 )
                {
                  v18 = v17;
                  v17 = (__int64 *)*v17;
                }
                else
                {
                  v17 = (__int64 *)v17[2];
                }
              }
              if ( *((_BYTE *)v18 + 25) != 0 || v15 < v18[4] )
                v18 = *(__int64 **)(a1 + 1120);
              v19 = v18 + 5;
              if ( v18 == v16 )
                v19 = nullptr;
              if ( v19 == nullptr
                || (*((_DWORD *)v19 + 1) & 2) != 0 && (unsigned __int8)sub_146E9FA80(v19) == 0
                || *((_DWORD *)v19 + 8) >= v10 )
              {
                goto LABEL_45;
              }
              sub_146E9FBD0(v19, v11, 0);
              ++*((_DWORD *)v19 + 8);
            }
          }
          sub_140439000(&unk_14E6A5F88);
          LOBYTE(v20) = 1;
          sub_145316BA0(&unk_14E6A5F88, v20);
          sub_145985760(v31);
          __wind
          {
            v21 = *(_QWORD *)(a1 + 160);
            if ( v21 != 0 && *(_DWORD *)(v21 + 8) != 0 )
              v22 = *(_QWORD *)(a1 + 168);
            else
              v22 = 0;
            v23 = v22 - 48;
            if ( v22 == 0 )
              v23 = 0;
            v31[0] = v23;
            v32 = 109128466;
            v33 = *(_DWORD *)(a1 + 144);
            v34 = sub_145B8C7D0(v5);
            v35 = sub_145B8C890(v5) + 1;
            v24 = (*(int (__fastcall **)(__int64))(*(_QWORD *)v5 + 832LL))(v5) / 2;
            v36 = v24 + sub_145B8C8C0(v5);
            v37 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v5 + 736LL))(v5);
            v39 = sub_140193D40((__int64)&unk_14E6A5F88);
            v40 = __crt_win32_buffer_debug_info::file_name((__crt_win32_buffer_debug_info *)&unk_14E6A5F88);
            v25 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 816LL))(a1);
            v42 = sub_145F73510(v25);
            v41 = 256;
            v38 = v5;
            sub_145985C70(v31);
          }
          __unwind
          {
            sub_1401DDB70(v31);
          }
          sub_1401DDB70(v31);
        }
        __unwind
        {
          sub_14017AC10(&v28);
        }
LABEL_45:
        v26 = v28;
        if ( (_QWORD)v28 != 0 )
        {
          v27 = (v29 - v28) & 0xFFFFFFFFFFFFFFF8uLL;
          if ( v27 >= 0x1000 )
          {
            v27 += 39LL;
            v26 = *(_QWORD *)(v28 - 8);
            if ( (unsigned __int64)(v28 - v26 - 8) > 0x1F )
              invalid_parameter_noinfo_noreturn();
          }
          j_j_scalable_free(v26, v27);
          v28 = 0;
          v29 = 0;
        }
      }
    }
  }
}
