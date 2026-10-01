// sub_140B90FA0  va=0x140B90FA0  size=1579

__int64 __fastcall sub_140B90FA0(__int64 a1, signed int a2, int a3, unsigned int a4, __int64 a5)
{
  __int64 v7; // rax
  __int64 v8; // rbx
  int v9; // r15d
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // rbx
  __int64 v13; // rsi
  __int64 v14; // rax
  int *v15; // rbx
  __int64 v16; // rcx
  int v17; // r8d
  int v18; // r10d
  __int64 v19; // rbx
  __int64 v20; // rax
  __int64 v21; // r9
  char v22; // al
  __int64 v23; // rax
  int *v24; // r14
  __int64 v25; // rax
  __int64 v26; // rax
  __int64 *v27; // rdx
  __int64 *v28; // rax
  __int64 *v29; // rcx
  int v30; // esi
  __int64 v31; // rbx
  __int64 v32; // rcx
  __int64 v33; // r8
  __int64 v34; // r9
  int v35; // edx
  char v36; // al
  char v37; // al
  __int64 v38; // rax
  __int64 v39; // rax
  int v40; // ebx
  int v41; // edx
  __int64 v42; // rcx
  int v43; // eax
  unsigned int v44; // eax
  char v45; // di
  int v46; // esi
  __int64 v47; // r14
  __int64 v48; // r8
  char v49; // al
  char v50; // al
  __int64 v51; // rax
  unsigned int v52; // edi
  __int64 v53; // rax
  __int64 v54; // r14
  __int64 v55; // rax
  int *v56; // rbx
  int v57; // ebx
  __int64 v58; // rax
  __int64 v59; // rcx
  __int64 *v60; // rbx
  __int64 *v61; // rcx
  __int64 v63; // [rsp+38h] [rbp-41h] BYREF
  __int64 v64; // [rsp+40h] [rbp-39h]
  _QWORD v65[4]; // [rsp+48h] [rbp-31h] BYREF
  __int64 *v66; // [rsp+68h] [rbp-11h] BYREF
  __int64 v67; // [rsp+70h] [rbp-9h]
  __int64 v68; // [rsp+78h] [rbp-1h]
  __int64 v69; // [rsp+80h] [rbp+7h]
  unsigned __int64 v70; // [rsp+88h] [rbp+Fh]
  __int64 v71; // [rsp+90h] [rbp+17h]
  __int64 v72; // [rsp+98h] [rbp+1Fh]

  v72 = -2;
  v7 = sub_145A70830((unsigned int)a2);
  if ( v7 == 0 )
    return 3;
  if ( *(_DWORD *)(v7 + 2120) != 36 )
    return 3;
  v8 = sub_145EFAFB0();
  v71 = v8;
  if ( v8 == 0 )
    return 3;
  v9 = 0;
  v64 = 0;
  v10 = sub_146E8BA20(40);
  *(_QWORD *)v10 = v10;
  *(_QWORD *)(v10 + 8) = v10;
  *(_QWORD *)(v10 + 16) = v10;
  *(_WORD *)(v10 + 24) = 257;
  v63 = v10;
  __wind
  {
    v24 = &dword_14DC641C8;
    while ( 1 )
    {
      v11 = (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v8 + 7944LL))(v8, (unsigned int)*v24);
      v12 = v11;
      if ( v11 != 0 && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v11 + 152LL))(v11) != 0 )
      {
        v13 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v12 + 1080LL))(v12);
        if ( v13 != 0 )
        {
          v14 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v12 + 152LL))(v12);
          v15 = (int *)(v14 + 24);
          sub_1480A6620(v14 + 24, 4, 50, 1, (_DWORD *)(v14 + 28));
          v17 = *v15;
          v18 = *(unsigned __int8 *)(v13 + 285);
          LODWORD(v68) = *v15;
          HIDWORD(v68) = v18;
          v19 = v63;
          v20 = *(_QWORD *)(v63 + 8);
          v65[0] = v20;
          LODWORD(v65[1]) = 0;
          v21 = v63;
          while ( *(_BYTE *)(v20 + 25) == 0 )
          {
            v65[0] = v20;
            if ( *(_DWORD *)(v20 + 28) == v17 )
            {
              if ( *(_DWORD *)(v20 + 32) == v18 )
              {
                LOBYTE(v16) = 0;
              }
              else
              {
                v16 = 255;
                if ( *(_DWORD *)(v20 + 32) >= v18 )
                  v16 = 1;
              }
            }
            else
            {
              v16 = 1;
              if ( *(_DWORD *)(v20 + 28) < v17 )
                v16 = 255;
            }
            if ( (v16 & 0x80u) == 0LL )
            {
              LODWORD(v65[1]) = 1;
              v21 = v20;
              v20 = *(_QWORD *)v20;
            }
            else
            {
              LODWORD(v65[1]) = 0;
              v20 = *(_QWORD *)(v20 + 16);
            }
          }
          if ( *(_BYTE *)(v21 + 25) != 0 )
            goto LABEL_31;
          if ( v17 == *(_DWORD *)(v21 + 28) )
          {
            if ( v18 == *(_DWORD *)(v21 + 32) )
            {
              v22 = 0;
            }
            else
            {
              v22 = -1;
              if ( v18 >= *(_DWORD *)(v21 + 32) )
                v22 = 1;
            }
          }
          else
          {
            v22 = 1;
            if ( v17 < *(_DWORD *)(v21 + 28) )
              v22 = -1;
          }
          if ( v22 < 0 )
          {
LABEL_31:
            if ( v64 == 0x666666666666666LL )
              unknown_libname_7(v16);
            v66 = &v63;
            v67 = 0;
            __wind
            {
              v67 = 0;
              v23 = sub_146E8BA20(40);
              v67 = v23;
            }
            __unwind
            {
              sub_140151030(&v66);
            }
            __wind
            {
              *(_QWORD *)(v23 + 28) = v68;
              *(_DWORD *)(v23 + 36) = 0;
              *(_QWORD *)v23 = v19;
              *(_QWORD *)(v23 + 8) = v19;
              *(_QWORD *)(v23 + 16) = v19;
              *(_WORD *)(v23 + 24) = 0;
            }
            __unwind
            {
              sub_1401511A0(&v66);
            }
            __wind
            {
              v67 = 0;
            }
            __unwind
            {
              sub_140151120(&v66);
            }
            v21 = sub_14014F0E0(&v63, v65, v23);
          }
          ++*(_DWORD *)(v21 + 36);
        }
      }
      if ( ++v24 == &dword_14DC641F4 )
        break;
      v8 = v71;
    }
    v25 = qword_14E6399A0;
    if ( qword_14E6399A0 == 0 )
    {
      v26 = sub_146E8BA20(1360);
      v68 = v26;
      __wind
      {
        if ( v26 != 0 )
          v25 = sub_140B8D280(v26);
        else
          v25 = 0;
      }
      __unwind
      {
        j_j_scalable_free(v68, 1360);
      }
      qword_14E6399A0 = v25;
    }
    if ( a2 == -1 || a2 == 0 )
      goto LABEL_54;
    v27 = *(__int64 **)(v25 + 632);
    v28 = (__int64 *)v27[1];
    v29 = v27;
    while ( *((_BYTE *)v28 + 25) == 0 )
    {
      if ( *((_DWORD *)v28 + 7) >= a2 )
      {
        v29 = v28;
        v28 = (__int64 *)*v28;
      }
      else
      {
        v28 = (__int64 *)v28[2];
      }
    }
    if ( *((_BYTE *)v29 + 25) != 0 || a2 < *((_DWORD *)v29 + 7) || v29 == v27 )
LABEL_54:
      v30 = 0;
    else
      v30 = *((_DWORD *)v29 + 8);
    v69 = (unsigned int)a2;
    v31 = v63;
    v32 = *(_QWORD *)(v63 + 8);
    v65[0] = v32;
    LODWORD(v65[1]) = 0;
    v33 = v63;
    v34 = 1;
    while ( *(_BYTE *)(v32 + 25) == 0 )
    {
      v65[0] = v32;
      if ( *(_DWORD *)(v32 + 28) == a2 )
      {
        v35 = *(_DWORD *)(v32 + 32);
        if ( v35 != 0 )
        {
          v36 = -1;
          if ( v35 >= 0 )
            v36 = 1;
        }
        else
        {
          v36 = 0;
        }
      }
      else
      {
        v36 = 1;
        if ( *(_DWORD *)(v32 + 28) < a2 )
          v36 = -1;
      }
      if ( v36 >= 0 )
      {
        LODWORD(v65[1]) = 1;
        v33 = v32;
        v32 = *(_QWORD *)v32;
      }
      else
      {
        LODWORD(v65[1]) = 0;
        v32 = *(_QWORD *)(v32 + 16);
      }
    }
    if ( *(_BYTE *)(v33 + 25) != 0 )
      goto LABEL_78;
    if ( a2 == *(_DWORD *)(v33 + 28) )
    {
      v32 = *(unsigned int *)(v33 + 32);
      if ( (_DWORD)v32 != 0 )
      {
        v37 = -1;
        if ( (int)v32 <= 0 )
          v37 = 1;
      }
      else
      {
        v37 = 0;
      }
    }
    else
    {
      v37 = 1;
      if ( a2 < *(_DWORD *)(v33 + 28) )
        v37 = -1;
    }
    if ( v37 < 0 )
    {
LABEL_78:
      if ( v64 == 0x666666666666666LL )
        goto LABEL_130;
      v66 = &v63;
      v67 = 0;
      __wind
      {
        v67 = 0;
        v38 = sub_146E8BA20(40);
        v67 = v38;
      }
      __unwind
      {
        sub_140151030(&v66);
      }
      __wind
      {
        *(_QWORD *)(v38 + 28) = v69;
        *(_DWORD *)(v38 + 36) = 0;
        *(_QWORD *)v38 = v31;
        *(_QWORD *)(v38 + 8) = v31;
        *(_QWORD *)(v38 + 16) = v31;
        *(_WORD *)(v38 + 24) = 0;
      }
      __unwind
      {
        sub_1401511A0(&v66);
      }
      __wind
      {
        v67 = 0;
      }
      __unwind
      {
        sub_140151120(&v66);
      }
      v39 = sub_14014F0E0(&v63, v65, v38);
      v33 = v39;
    }
    *(_DWORD *)(v33 + 36) += v30;
    v40 = 0;
    v41 = 0;
    v42 = a5;
    do
    {
      if ( *(_DWORD *)v42 == a2 )
      {
        v43 = *(unsigned __int8 *)(v42 + 4);
        if ( v43 == a3 && v41 != a4 )
        {
          v44 = *(_DWORD *)(v42 + 8) - 1;
          if ( v44 <= 1 )
            ++v40;
        }
      }
      ++v41;
      v42 += 32;
    }
    while ( v42 != a5 + 1504 );
    v45 = -1;
    v46 = 0;
    if ( a3 > -1 )
      v46 = a3;
    v70 = __PAIR64__(v46, a2);
    v47 = v63;
    v32 = *(_QWORD *)(v63 + 8);
    v65[0] = v32;
    LODWORD(v65[1]) = 0;
    v48 = v63;
    while ( *(_BYTE *)(v32 + 25) == 0 )
    {
      v65[0] = v32;
      if ( *(_DWORD *)(v32 + 28) == a2 )
      {
        if ( *(_DWORD *)(v32 + 32) == v46 )
        {
          v49 = 0;
        }
        else
        {
          v49 = -1;
          if ( *(_DWORD *)(v32 + 32) >= v46 )
            v49 = 1;
        }
      }
      else
      {
        v49 = 1;
        if ( *(_DWORD *)(v32 + 28) < a2 )
          v49 = -1;
      }
      if ( v49 >= 0 )
      {
        LODWORD(v65[1]) = 1;
        v48 = v32;
        v32 = *(_QWORD *)v32;
      }
      else
      {
        LODWORD(v65[1]) = 0;
        v32 = *(_QWORD *)(v32 + 16);
      }
    }
    if ( *(_BYTE *)(v48 + 25) == 0 )
    {
      if ( a2 == *(_DWORD *)(v48 + 28) )
      {
        if ( v46 == *(_DWORD *)(v48 + 32) )
        {
          v45 = 0;
        }
        else if ( v46 >= *(_DWORD *)(v48 + 32) )
        {
          v45 = 1;
        }
      }
      else
      {
        v50 = 1;
        if ( a2 < *(_DWORD *)(v48 + 28) )
          v50 = -1;
        v45 = v50;
      }
      if ( v45 >= 0 )
        goto LABEL_117;
    }
    if ( v64 != 0x666666666666666LL )
    {
      v66 = &v63;
      v67 = 0;
      __wind
      {
        v67 = 0;
        v51 = sub_146E8BA20(40);
        v67 = v51;
      }
      __unwind
      {
        sub_140151030(&v66);
      }
      __wind
      {
        *(_QWORD *)(v51 + 28) = v70;
        *(_DWORD *)(v51 + 36) = 0;
        *(_QWORD *)v51 = v47;
        *(_QWORD *)(v51 + 8) = v47;
        *(_QWORD *)(v51 + 16) = v47;
        *(_WORD *)(v51 + 24) = 0;
      }
      __unwind
      {
        sub_1401511A0(&v66);
      }
      __wind
      {
        v67 = 0;
      }
      __unwind
      {
        sub_140151120(&v66);
      }
      v48 = sub_14014F0E0(&v63, v65, v51);
LABEL_117:
      v52 = (*(_DWORD *)(v48 + 36) - v40 <= 0) + 2;
      if ( *(_DWORD *)(v48 + 36) - v40 > 0 )
      {
        v53 = (*(__int64 (__fastcall **)(__int64, _QWORD, __int64, __int64))(*(_QWORD *)v71 + 7944LL))(
                v71,
                a4,
                v48,
                v34);
        v54 = v53;
        if ( v53 != 0 && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v53 + 152LL))(v53) != 0 )
        {
          v55 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v54 + 152LL))(v54);
          v56 = (int *)(v55 + 24);
          sub_1480A6620(v55 + 24, 4, 50, 1, (_DWORD *)(v55 + 28));
          v57 = *v56;
          v58 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v54 + 1080LL))(v54);
          if ( v58 != 0 )
            v9 = *(unsigned __int8 *)(v58 + 285);
          if ( v57 == a2 && v9 == v46 )
            v52 = 1;
        }
      }
      goto LABEL_132;
    }
LABEL_130:
    unknown_libname_7(v32);
  }
  __unwind
  {
    sub_140151260(&v63);
  }
LABEL_132:
  v59 = v63;
  v60 = *(__int64 **)(v63 + 8);
  if ( *((_BYTE *)v60 + 25) == 0 )
  {
    do
    {
      sub_140150890(&v63, &v63, v60[2]);
      v61 = v60;
      v60 = (__int64 *)*v60;
      j_j_scalable_free(v61, 40);
    }
    while ( *((_BYTE *)v60 + 25) == 0 );
    v59 = v63;
  }
  j_j_scalable_free(v59, 40);
  return v52;
}
