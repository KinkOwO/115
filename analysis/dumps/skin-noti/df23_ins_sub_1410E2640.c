// ins_sub_1410E2640

__int64 __fastcall sub_1410E2640(__int64 a1, __int64 a2)
{
  __int64 v4; // rax
  __int64 result; // rax
  unsigned int **v6; // r14
  __int64 v7; // rbx
  __int64 v8; // rax
  unsigned int *v9; // rbx
  int v10; // r9d
  int v11; // r8d
  __int64 v12; // rax
  _DWORD *v13; // rbx
  __int64 v14; // rax
  __int64 v15; // r8
  volatile signed __int32 *v16; // rcx
  volatile signed __int32 *v17; // rbx
  __int64 v18; // rbp
  __int64 v19; // r8
  __int64 v20; // rax
  __int64 v21; // rbx
  unsigned int v22; // eax
  __int64 v23; // rax
  _QWORD *v24; // rdx
  unsigned int v25; // ecx
  int v26; // eax
  __int64 v27; // rax
  __int64 v28; // rbx
  _QWORD *v29; // rdx
  unsigned int v30; // ecx
  int v31; // eax
  volatile signed __int32 *v32; // [rsp+48h] [rbp-50h]
  __int128 v33; // [rsp+50h] [rbp-48h] BYREF
  __int128 v34; // [rsp+60h] [rbp-38h]
  int v35; // [rsp+A8h] [rbp+10h] BYREF
  _DWORD *v36; // [rsp+B8h] [rbp+20h]

  if ( a2 && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a2 + 152LL))(a2) )
  {
    v33 = 0;
    v6 = (unsigned int **)(a1 + 168);
    sub_1401E5080(a1 + 168, &v33);
    if ( *((_QWORD *)&v33 + 1) )
    {
      if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v33 + 1) + 8LL), 0xFFFFFFFF) == 1 )
      {
        v7 = *((_QWORD *)&v33 + 1);
        (***((void (__fastcall ****)(_QWORD))&v33 + 1))(*((_QWORD *)&v33 + 1));
        if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v7 + 12), 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(_QWORD))(**((_QWORD **)&v33 + 1) + 8LL))(*((_QWORD *)&v33 + 1));
      }
    }
    v8 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a2 + 152LL))(a2);
    v9 = (unsigned int *)(v8 + 24);
    LOBYTE(v10) = 1;
    LOBYTE(v11) = 50;
    sub_1480A6620(v8 + 24, 4, v11, v10, v8 + 28);
    sub_1414774A0(&v35, *v9);
    v12 = sub_146E8BA20(248);
    if ( v12 )
      v13 = (_DWORD *)sub_14586BD00(v12);
    else
      v13 = 0;
    v36 = v13;
    v14 = sub_146E8BA20(24);
    v16 = (volatile signed __int32 *)v14;
    if ( v14 )
    {
      *(_OWORD *)v14 = 0;
      *(_DWORD *)(v14 + 8) = 1;
      *(_DWORD *)(v14 + 12) = 1;
      *(_QWORD *)v14 = off_1495CF088;
      *(_QWORD *)(v14 + 16) = v13;
    }
    else
    {
      v16 = 0;
    }
    v32 = v16;
    v36 = 0;
    *v13 = v35;
    v34 = 0;
    if ( v16 )
      _InterlockedIncrement(v16 + 2);
    *(_QWORD *)&v34 = v13;
    *((_QWORD *)&v34 + 1) = v16;
    *(_QWORD *)&v34 = *v6;
    *v6 = v13;
    *((_QWORD *)&v34 + 1) = *(_QWORD *)(a1 + 176);
    v17 = (volatile signed __int32 *)*((_QWORD *)&v34 + 1);
    *(_QWORD *)(a1 + 176) = v16;
    if ( v17 )
    {
      if ( _InterlockedExchangeAdd(v17 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v17)(v17);
        if ( _InterlockedExchangeAdd(v17 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v17 + 8LL))(v17);
      }
    }
    LOBYTE(v15) = 1;
    v18 = sub_14021BE90(qword_14E683B30, **v6, v15);
    if ( v18 )
    {
      v20 = sub_1405BA800();
      if ( (unsigned __int8)sub_142D4BB10(v20, a2) )
      {
        v21 = sub_1405BA800();
        v22 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a2 + 936LL))(a2);
        LODWORD(v21) = sub_142D46470(v21, v22);
        v23 = sub_1405BA800();
        if ( (unsigned __int8)sub_142D4BB00(v23, (unsigned int)v21) )
          (*v6)[30] = 1;
      }
      else
      {
        (*v6)[30] = 0;
      }
      v24 = (_QWORD *)(v18 + 496);
      if ( a1 + 104 != v18 + 496 )
      {
        if ( *(_QWORD *)(v18 + 520) >= 8u )
          v24 = (_QWORD *)*v24;
        sub_14014C8D0(a1 + 104, v24);
      }
      v25 = *(_DWORD *)(v18 + 12);
      *(_BYTE *)(a1 + 152) = v25 <= 8 && (v26 = 329, _bittest(&v26, v25));
    }
    else
    {
      LOBYTE(v19) = 1;
      v27 = sub_140283D60(qword_14E683B38, **v6, v19);
      v28 = v27;
      if ( v27 )
      {
        v29 = (_QWORD *)(v27 + 496);
        if ( a1 + 104 != v27 + 496 )
        {
          if ( *(_QWORD *)(v27 + 520) >= 8u )
            v29 = (_QWORD *)*v29;
          sub_14014C8D0(a1 + 104, v29);
        }
        v30 = *(_DWORD *)(v28 + 12);
        *(_BYTE *)(a1 + 152) = v30 <= 8 && (v31 = 329, _bittest(&v31, v30));
      }
    }
    result = sub_1410E57A0(a1);
    if ( v32 )
    {
      result = (unsigned int)_InterlockedExchangeAdd(v32 + 2, 0xFFFFFFFF);
      if ( (_DWORD)result == 1 )
      {
        result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v32)(v32);
        if ( _InterlockedExchangeAdd(v32 + 3, 0xFFFFFFFF) == 1 )
          return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v32 + 8LL))(v32);
      }
    }
  }
  else
  {
    v4 = sub_14723C170(100087647);
    sub_14668C520(qword_14E683C78, 2875, v4, 0);
    return sub_1410E57A0(a1);
  }
  return result;
}

